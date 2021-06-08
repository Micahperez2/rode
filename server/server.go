// Copyright 2021 The Rode Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rode/rode/pkg/policy"
	"github.com/rode/rode/pkg/resource"
	"github.com/rode/rode/protodeps/grafeas/proto/v1beta1/common_go_proto"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/rode/es-index-manager/indexmanager"

	"github.com/rode/grafeas-elasticsearch/go/v1beta1/storage/esutil"
	"github.com/rode/grafeas-elasticsearch/go/v1beta1/storage/filtering"
	pb "github.com/rode/rode/proto/v1alpha1"
	grafeas_proto "github.com/rode/rode/protodeps/grafeas/proto/v1beta1/grafeas_go_proto"
	grafeas_project_proto "github.com/rode/rode/protodeps/grafeas/proto/v1beta1/project_go_proto"

	"github.com/golang/protobuf/proto"
	"github.com/rode/rode/config"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	rodeProjectSlug                   = "projects/rode"
	rodeElasticsearchOccurrencesAlias = "grafeas-rode-occurrences"
	policiesDocumentKind              = "policies"
	genericResourcesDocumentKind      = "generic-resources"
	maxPageSize                       = 1000
	pitKeepAlive                      = "5m"
)

// NewRodeServer constructor for rodeServer
func NewRodeServer(
	logger *zap.Logger,
	grafeasCommon grafeas_proto.GrafeasV1Beta1Client,
	grafeasProjects grafeas_project_proto.ProjectsClient,
	esClient *elasticsearch.Client,
	filterer filtering.Filterer,
	elasticsearchConfig *config.ElasticsearchConfig,
	resourceManager resource.Manager,
	indexManager indexmanager.IndexManager,
	policyManager policy.Manager,
) (pb.RodeServer, error) {
	rodeServer := &rodeServer{
		logger,
		esClient,
		filterer,
		grafeasCommon,
		grafeasProjects,
		elasticsearchConfig,
		resourceManager,
		indexManager,
		policyManager,
	}

	if err := rodeServer.initialize(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize rode server: %s", err)
	}

	return rodeServer, nil
}

type rodeServer struct {
	logger              *zap.Logger
	esClient            *elasticsearch.Client
	filterer            filtering.Filterer
	grafeasCommon       grafeas_proto.GrafeasV1Beta1Client
	grafeasProjects     grafeas_project_proto.ProjectsClient
	elasticsearchConfig *config.ElasticsearchConfig
	resourceManager     resource.Manager
	indexManager        indexmanager.IndexManager
	policy.Manager
}

func (r *rodeServer) BatchCreateOccurrences(ctx context.Context, occurrenceRequest *pb.BatchCreateOccurrencesRequest) (*pb.BatchCreateOccurrencesResponse, error) {
	log := r.logger.Named("BatchCreateOccurrences")
	log.Debug("received request", zap.Any("BatchCreateOccurrencesRequest", occurrenceRequest))

	occurrenceResponse, err := r.grafeasCommon.BatchCreateOccurrences(ctx, &grafeas_proto.BatchCreateOccurrencesRequest{
		Parent:      rodeProjectSlug,
		Occurrences: occurrenceRequest.GetOccurrences(),
	})
	if err != nil {
		return nil, createError(log, "error creating occurrences", err)
	}

	if err = r.resourceManager.BatchCreateGenericResources(ctx, occurrenceResponse.Occurrences); err != nil {
		return nil, createError(log, "error creating generic resources", err)
	}

	if err = r.resourceManager.BatchCreateGenericResourceVersions(ctx, occurrenceResponse.Occurrences); err != nil {
		return nil, createError(log, "error creating generic resource versions", err)
	}

	return &pb.BatchCreateOccurrencesResponse{
		Occurrences: occurrenceResponse.GetOccurrences(),
	}, nil
}

func (r *rodeServer) ListResources(ctx context.Context, request *pb.ListResourcesRequest) (*pb.ListResourcesResponse, error) {
	log := r.logger.Named("ListResources")
	log.Debug("received request", zap.Any("ListResourcesRequest", request))

	hits, nextPageToken, err := r.genericList(ctx, log, &genericListOptions{
		index:     rodeElasticsearchOccurrencesAlias,
		filter:    request.Filter,
		pageSize:  request.PageSize,
		pageToken: request.PageToken,
		query: &esutil.EsSearch{
			Collapse: &esutil.EsSearchCollapse{
				Field: "resource.uri",
			},
		},
		sortDirection: esutil.EsSortOrderAscending,
		sortField:     "resource.uri",
	})

	if err != nil {
		return nil, err
	}

	var resources []*grafeas_proto.Resource
	for _, hit := range hits.Hits {
		occurrence := &grafeas_proto.Occurrence{}
		err := protojson.Unmarshal(hit.Source, proto.MessageV2(occurrence))
		if err != nil {
			return nil, createError(log, "error unmarshalling search result", err)
		}

		resources = append(resources, occurrence.Resource)
	}

	return &pb.ListResourcesResponse{
		Resources:     resources,
		NextPageToken: nextPageToken,
	}, nil
}

func (r *rodeServer) ListGenericResources(ctx context.Context, request *pb.ListGenericResourcesRequest) (*pb.ListGenericResourcesResponse, error) {
	log := r.logger.Named("ListGenericResources")
	log.Debug("received request", zap.Any("request", request))

	return r.resourceManager.ListGenericResources(ctx, request)
}

func (r *rodeServer) ListGenericResourceVersions(ctx context.Context, request *pb.ListGenericResourceVersionsRequest) (*pb.ListGenericResourceVersionsResponse, error) {
	log := r.logger.Named("ListGenericResourceVersions").With(zap.Any("resource", request.Id))

	if request.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "resource id is required")
	}

	genericResource, err := r.resourceManager.GetGenericResource(ctx, request.Id)
	if err != nil {
		return nil, err
	}

	if genericResource == nil {
		log.Debug("generic resource not found")

		return nil, status.Error(codes.NotFound, fmt.Sprintf("generic resource with id %s not found", request.Id))
	}

	return r.resourceManager.ListGenericResourceVersions(ctx, request)
}

func (r *rodeServer) initialize(ctx context.Context) error {
	log := r.logger.Named("initialize")

	if err := r.indexManager.Initialize(ctx); err != nil {
		return fmt.Errorf("error initializing index manager: %s", err)
	}

	_, err := r.grafeasProjects.GetProject(ctx, &grafeas_project_proto.GetProjectRequest{Name: rodeProjectSlug})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			_, err := r.grafeasProjects.CreateProject(ctx, &grafeas_project_proto.CreateProjectRequest{Project: &grafeas_project_proto.Project{Name: rodeProjectSlug}})
			if err != nil {
				log.Error("failed to create rode project", zap.Error(err))
				return err
			}
			log.Info("created rode project")
		} else {
			log.Error("error checking if rode project exists", zap.Error(err))
			return err
		}
	}

	indexSettings := []struct {
		indexName    string
		aliasName    string
		documentKind string
	}{
		{
			indexName:    r.indexManager.IndexName(policiesDocumentKind, ""),
			aliasName:    r.indexManager.AliasName(policiesDocumentKind, ""),
			documentKind: policiesDocumentKind,
		},
		{
			indexName:    r.indexManager.IndexName(genericResourcesDocumentKind, ""),
			aliasName:    r.indexManager.AliasName(genericResourcesDocumentKind, ""),
			documentKind: genericResourcesDocumentKind,
		},
	}

	for _, settings := range indexSettings {
		if err := r.indexManager.CreateIndex(ctx, settings.indexName, settings.aliasName, settings.documentKind); err != nil {
			return fmt.Errorf("error creating index: %s", err)
		}
	}

	return nil
}

func (r *rodeServer) ListVersionedResourceOccurrences(ctx context.Context, request *pb.ListVersionedResourceOccurrencesRequest) (*pb.ListVersionedResourceOccurrencesResponse, error) {
	log := r.logger.Named("ListVersionedResourceOccurrences")
	log.Debug("received request", zap.Any("ListVersionedResourceOccurrencesRequest", request))

	resourceUri := request.ResourceUri
	if resourceUri == "" {
		return nil, createErrorWithCode(log, "invalid request", errors.New("must set resource_uri"), codes.InvalidArgument)
	}

	log.Debug("listing build occurrences")
	buildOccurrences, err := r.grafeasCommon.ListOccurrences(ctx, &grafeas_proto.ListOccurrencesRequest{
		Parent:   rodeProjectSlug,
		PageSize: maxPageSize,
		Filter:   fmt.Sprintf(`kind == "BUILD" && (resource.uri == "%[1]s" || build.provenance.builtArtifacts.nestedFilter(id == "%[1]s"))`, resourceUri),
	})
	if err != nil {
		return nil, createError(log, "error fetching build occurrences", err)
	}

	resourceUris := map[string]string{
		resourceUri: resourceUri,
	}
	for _, occurrence := range buildOccurrences.Occurrences {
		resourceUris[occurrence.Resource.Uri] = occurrence.Resource.Uri
		for _, artifact := range occurrence.GetBuild().GetProvenance().BuiltArtifacts {
			resourceUris[artifact.Id] = artifact.Id
		}
	}

	var resourceFilters []string
	for k := range resourceUris {
		resourceFilters = append(resourceFilters, fmt.Sprintf(`resource.uri == "%s"`, k))
	}

	filter := strings.Join(resourceFilters, " || ")
	log.Debug("listing occurrences", zap.String("filter", filter))
	allOccurrences, err := r.grafeasCommon.ListOccurrences(ctx, &grafeas_proto.ListOccurrencesRequest{
		Parent:    rodeProjectSlug,
		Filter:    filter,
		PageSize:  request.PageSize,
		PageToken: request.PageToken,
	})
	if err != nil {
		return nil, createError(log, "error listing occurrences", err)
	}

	response := &pb.ListVersionedResourceOccurrencesResponse{
		Occurrences:   allOccurrences.Occurrences,
		NextPageToken: allOccurrences.NextPageToken,
	}

	if request.FetchRelatedNotes {
		relatedNotes, err := r.fetchRelatedNotes(ctx, log, allOccurrences.Occurrences)
		if err != nil {
			return nil, createError(log, "error fetching related notes", err)
		}

		response.RelatedNotes = relatedNotes
	}

	return response, nil
}

func (r *rodeServer) fetchRelatedNotes(ctx context.Context, logger *zap.Logger, occurrences []*grafeas_proto.Occurrence) (map[string]*grafeas_proto.Note, error) {
	log := logger.Named("fetchRelatedNotes")

	if len(occurrences) == 0 {
		return nil, nil
	}

	noteFiltersMap := make(map[string]string)
	for _, occurrence := range occurrences {
		if _, ok := noteFiltersMap[occurrence.NoteName]; !ok {
			noteFiltersMap[occurrence.NoteName] = fmt.Sprintf(`"name" == "%s"`, occurrence.NoteName)
		}
	}

	var noteFilters []string
	for _, filter := range noteFiltersMap {
		noteFilters = append(noteFilters, filter)
	}

	log.Debug("fetching related notes")
	listNotesResponse, err := r.grafeasCommon.ListNotes(ctx, &grafeas_proto.ListNotesRequest{
		Parent:   rodeProjectSlug,
		Filter:   strings.Join(noteFilters, " || "),
		PageSize: maxPageSize,
	})
	if err != nil {
		return nil, err
	}

	result := make(map[string]*grafeas_proto.Note)
	for _, note := range listNotesResponse.Notes {
		result[note.Name] = note
	}

	return result, nil
}

func (r *rodeServer) ListOccurrences(ctx context.Context, occurrenceRequest *pb.ListOccurrencesRequest) (*pb.ListOccurrencesResponse, error) {
	log := r.logger.Named("ListOccurrences")
	log.Debug("received request", zap.Any("ListOccurrencesRequest", occurrenceRequest))

	request := &grafeas_proto.ListOccurrencesRequest{
		Parent:    rodeProjectSlug,
		Filter:    occurrenceRequest.Filter,
		PageToken: occurrenceRequest.PageToken,
		PageSize:  occurrenceRequest.PageSize,
	}

	listOccurrencesResponse, err := r.grafeasCommon.ListOccurrences(ctx, request)
	if err != nil {
		return nil, createError(log, "error listing occurrences", err)
	}

	return &pb.ListOccurrencesResponse{
		Occurrences:   listOccurrencesResponse.GetOccurrences(),
		NextPageToken: listOccurrencesResponse.GetNextPageToken(),
	}, nil
}

func (r *rodeServer) UpdateOccurrence(ctx context.Context, occurrenceRequest *pb.UpdateOccurrenceRequest) (*grafeas_proto.Occurrence, error) {
	log := r.logger.Named("UpdateOccurrence")
	log.Debug("received request", zap.Any("UpdateOccurrenceRequest", occurrenceRequest))

	name := fmt.Sprintf("projects/rode/occurrences/%s", occurrenceRequest.Id)

	if occurrenceRequest.Occurrence.Name != name {
		log.Error("occurrence name does not contain the occurrence id", zap.String("occurrenceName", occurrenceRequest.Occurrence.Name), zap.String("id", occurrenceRequest.Id))
		return nil, status.Error(codes.InvalidArgument, "occurrence name does not contain the occurrence id")
	}

	updatedOccurrence, err := r.grafeasCommon.UpdateOccurrence(ctx, &grafeas_proto.UpdateOccurrenceRequest{
		Name:       name,
		Occurrence: occurrenceRequest.Occurrence,
		UpdateMask: occurrenceRequest.UpdateMask,
	})
	if err != nil {
		return nil, createError(log, "error updating occurrence", err)
	}

	return updatedOccurrence, nil
}

func (r *rodeServer) RegisterCollector(ctx context.Context, registerCollectorRequest *pb.RegisterCollectorRequest) (*pb.RegisterCollectorResponse, error) {
	log := r.logger.Named("RegisterCollector")

	if registerCollectorRequest.Id == "" {
		return nil, createErrorWithCode(log, "collector ID is required", nil, codes.InvalidArgument)
	}

	if len(registerCollectorRequest.Notes) == 0 {
		return &pb.RegisterCollectorResponse{}, nil
	}

	// build collection of notes that potentially need to be created
	notesWithIds := make(map[string]*grafeas_proto.Note)
	notesToCreate := make(map[string]*grafeas_proto.Note)
	for _, note := range registerCollectorRequest.Notes {
		noteId := buildNoteIdFromCollectorId(registerCollectorRequest.Id, note)

		if _, ok := notesWithIds[noteId]; ok {
			return nil, createErrorWithCode(log, "cannot use more than one note type when registering a collector", nil, codes.InvalidArgument)
		}

		notesWithIds[noteId] = note
		notesToCreate[noteId] = note
	}

	log = log.With(zap.Any("notes", notesWithIds))

	// find out which notes already exist
	filter := fmt.Sprintf(`name.startsWith("%s/notes/%s-")`, rodeProjectSlug, registerCollectorRequest.Id)
	listNotesResponse, err := r.grafeasCommon.ListNotes(ctx, &grafeas_proto.ListNotesRequest{
		Parent: rodeProjectSlug,
		Filter: filter,
	})
	if err != nil {
		return nil, createError(log, "error listing notes", err)
	}

	// build map of notes that need to be created
	for _, note := range listNotesResponse.Notes {
		noteId := getNoteIdFromNoteName(note.Name)

		if _, ok := notesWithIds[noteId]; ok {
			notesWithIds[noteId].Name = note.Name
			delete(notesToCreate, noteId)
		}
	}

	if len(notesToCreate) != 0 {
		batchCreateNotesResponse, err := r.grafeasCommon.BatchCreateNotes(ctx, &grafeas_proto.BatchCreateNotesRequest{
			Parent: rodeProjectSlug,
			Notes:  notesToCreate,
		})
		if err != nil {
			return nil, createError(log, "error creating notes", err)
		}

		for _, note := range batchCreateNotesResponse.Notes {
			noteId := getNoteIdFromNoteName(note.Name)

			if _, ok := notesWithIds[noteId]; ok {
				notesWithIds[noteId].Name = note.Name
			}
		}
	}

	return &pb.RegisterCollectorResponse{
		Notes: notesWithIds,
	}, nil
}

// CreateNote operates as a simple proxy to grafeas.CreateNote, for now.
func (r *rodeServer) CreateNote(ctx context.Context, request *pb.CreateNoteRequest) (*grafeas_proto.Note, error) {
	log := r.logger.Named("CreateNote").With(zap.String("noteId", request.NoteId))

	log.Debug("creating note in grafeas")

	return r.grafeasCommon.CreateNote(ctx, &grafeas_proto.CreateNoteRequest{
		Parent: rodeProjectSlug,
		NoteId: request.NoteId,
		Note:   request.Note,
	})
}

type genericListOptions struct {
	index         string
	filter        string
	query         *esutil.EsSearch
	pageSize      int32
	pageToken     string
	sortDirection esutil.EsSortOrder
	sortField     string
}

func (r *rodeServer) genericList(ctx context.Context, log *zap.Logger, options *genericListOptions) (*esutil.EsSearchResponseHits, string, error) {
	body := &esutil.EsSearch{}
	if options.query != nil {
		body = options.query
	}

	if options.filter != "" {
		log = log.With(zap.String("filter", options.filter))
		filterQuery, err := r.filterer.ParseExpression(options.filter)
		if err != nil {
			return nil, "", createError(log, "error while parsing filter expression", err)
		}

		body.Query = filterQuery
	}

	if options.sortField != "" {
		body.Sort = map[string]esutil.EsSortOrder{
			options.sortField: options.sortDirection,
		}
	}

	searchOptions := []func(*esapi.SearchRequest){
		r.esClient.Search.WithContext(ctx),
	}

	var nextPageToken string
	if options.pageToken != "" || options.pageSize != 0 { // handle pagination
		next, extraSearchOptions, err := r.handlePagination(ctx, log, body, options.index, options.pageToken, options.pageSize)
		if err != nil {
			return nil, "", createError(log, "error while handling pagination", err)
		}

		nextPageToken = next
		searchOptions = append(searchOptions, extraSearchOptions...)
	} else {
		searchOptions = append(searchOptions,
			r.esClient.Search.WithIndex(options.index),
			r.esClient.Search.WithSize(maxPageSize),
		)
	}

	encodedBody, requestJson := esutil.EncodeRequest(body)
	log = log.With(zap.String("request", requestJson))
	log.Debug("performing search")

	res, err := r.esClient.Search(
		append(searchOptions, r.esClient.Search.WithBody(encodedBody))...,
	)
	if err != nil {
		return nil, "", createError(log, "error sending request to elasticsearch", err)
	}
	if res.IsError() {
		return nil, "", createError(log, "unexpected response from elasticsearch", nil, zap.String("response", res.String()), zap.Int("status", res.StatusCode))
	}

	var searchResults esutil.EsSearchResponse
	if err := esutil.DecodeResponse(res.Body, &searchResults); err != nil {
		return nil, "", createError(log, "error decoding elasticsearch response", err)
	}

	if options.pageToken != "" || options.pageSize != 0 { // if request is paginated, check for last page
		_, from, err := esutil.ParsePageToken(nextPageToken)
		if err != nil {
			return nil, "", createError(log, "error parsing page token", err)
		}

		if from >= searchResults.Hits.Total.Value {
			nextPageToken = ""
		}
	}
	return searchResults.Hits, nextPageToken, nil
}

func (r *rodeServer) handlePagination(ctx context.Context, log *zap.Logger, body *esutil.EsSearch, index, pageToken string, pageSize int32) (string, []func(*esapi.SearchRequest), error) {
	log = log.With(zap.String("pageToken", pageToken), zap.Int32("pageSize", pageSize))

	var (
		pit  string
		from int
		err  error
	)

	// if no pageToken is specified, we need to create a new PIT
	if pageToken == "" {
		res, err := r.esClient.OpenPointInTime(
			r.esClient.OpenPointInTime.WithContext(ctx),
			r.esClient.OpenPointInTime.WithIndex(index),
			r.esClient.OpenPointInTime.WithKeepAlive(pitKeepAlive),
		)
		if err != nil {
			return "", nil, createError(log, "error sending request to elasticsearch", err)
		}
		if res.IsError() {
			return "", nil, createError(log, "unexpected response from elasticsearch", nil, zap.String("response", res.String()), zap.Int("status", res.StatusCode))
		}

		var pitResponse esutil.ESPitResponse
		if err = esutil.DecodeResponse(res.Body, &pitResponse); err != nil {
			return "", nil, createError(log, "error decoding elasticsearch response", err)
		}

		pit = pitResponse.Id
		from = 0
	} else {
		// get the PIT from the provided pageToken
		pit, from, err = esutil.ParsePageToken(pageToken)
		if err != nil {
			return "", nil, createError(log, "error parsing page token", err)
		}
	}

	body.Pit = &esutil.EsSearchPit{
		Id:        pit,
		KeepAlive: pitKeepAlive,
	}

	return esutil.CreatePageToken(pit, from+int(pageSize)), []func(*esapi.SearchRequest){
		r.esClient.Search.WithSize(int(pageSize)),
		r.esClient.Search.WithFrom(from),
	}, err
}

// createError is a helper function that allows you to easily log an error and return a gRPC formatted error.
func createError(log *zap.Logger, message string, err error, fields ...zap.Field) error {
	return createErrorWithCode(log, message, err, codes.Internal, fields...)
}

// createError is a helper function that allows you to easily log an error and return a gRPC formatted error.
func createErrorWithCode(log *zap.Logger, message string, err error, code codes.Code, fields ...zap.Field) error {
	if err == nil {
		log.Error(message, fields...)
		return status.Errorf(code, "%s", message)
	}

	log.Error(message, append(fields, zap.Error(err))...)
	return status.Errorf(code, "%s: %s", message, err)
}

func buildNoteIdFromCollectorId(collectorId string, note *grafeas_proto.Note) string {
	switch note.Kind {
	case common_go_proto.NoteKind_VULNERABILITY:
		return fmt.Sprintf("%s-vulnerability", collectorId)
	case common_go_proto.NoteKind_BUILD:
		return fmt.Sprintf("%s-build", collectorId)
	case common_go_proto.NoteKind_IMAGE:
		return fmt.Sprintf("%s-image", collectorId)
	case common_go_proto.NoteKind_PACKAGE:
		return fmt.Sprintf("%s-package", collectorId)
	case common_go_proto.NoteKind_DEPLOYMENT:
		return fmt.Sprintf("%s-deployment", collectorId)
	case common_go_proto.NoteKind_DISCOVERY:
		return fmt.Sprintf("%s-discovery", collectorId)
	case common_go_proto.NoteKind_ATTESTATION:
		return fmt.Sprintf("%s-attestation", collectorId)
	case common_go_proto.NoteKind_INTOTO:
		return fmt.Sprintf("%s-intoto", collectorId)
	}

	return fmt.Sprintf("%s-unspecified", collectorId)
}

func getNoteIdFromNoteName(noteName string) string {
	// note name format: projects/${projectId}/notes/${noteId}
	return strings.TrimPrefix(noteName, rodeProjectSlug+"/notes/")
}
