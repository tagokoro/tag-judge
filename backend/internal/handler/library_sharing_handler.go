package handler

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/usecase"
)

// maxLibraryRequestBytes bounds the raw JSON body for library-sharing endpoints
// that accept a large library version source. The version usecase caps a
// decoded source at domain.MaxLibrarySourceBytes bytes, but JSON escaping can
// expand one decoded byte to as many as 6 raw bytes (a \uXXXX escape), so a
// fully escaped valid body is still admitted while memory stays bounded.
// Submission URL requests carry only a short URL but share this decoder.
// Submission and code-run endpoints keep decodeJSON's original unbounded
// contract.
const maxLibraryRequestBytes = 6*domain.MaxLibrarySourceBytes + 64*1024

// decodeLibraryJSON bounds a request body before the shared decoder runs so an
// oversized body gets a stable 400 rather than being silently truncated.
func decodeLibraryJSON(r *http.Request, target any) error {
	data, err := io.ReadAll(io.LimitReader(r.Body, maxLibraryRequestBytes+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > maxLibraryRequestBytes {
		return errors.New("request body too large")
	}
	r.Body = io.NopCloser(bytes.NewReader(data))
	return decodeJSON(r, target)
}

type CategoryHandler struct {
	categories *usecase.CategoryUsecase
}

func NewCategoryHandler(categories *usecase.CategoryUsecase) *CategoryHandler {
	return &CategoryHandler{categories: categories}
}

func (h *CategoryHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	categories, err := h.categories.ListMine(r.Context(), actor)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, newCategoryListResponse(categories))
}

func (h *CategoryHandler) ListByHandle(w http.ResponseWriter, r *http.Request) {
	limit, limitProvided, err := parseOptionalInt(r, "limit")
	if err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_LIMIT", "limit must be an integer"))
		return
	}
	offset, _, err := parseOptionalInt(r, "offset")
	if err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_OFFSET", "offset must be an integer"))
		return
	}
	actor, _ := auth.ActorFromContext(r.Context())

	categories, err := h.categories.ListByHandle(r.Context(), actor, chi.URLParam(r, "handle"), limit, limitProvided, offset)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, newCategoryListResponse(categories))
}

func (h *CategoryHandler) Get(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	category, err := h.categories.Get(r.Context(), actor, chi.URLParam(r, "category_id"))
	if err != nil {
		WriteError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, newCategoryResponse(*category))
}

func (h *CategoryHandler) Update(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	var request struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}
	if err := decodeLibraryJSON(r, &request); err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_REQUEST_BODY", "request body must be valid JSON"))
		return
	}

	category, err := h.categories.Update(r.Context(), actor, chi.URLParam(r, "category_id"), usecase.UpdateCategoryInput{
		Name:        request.Name,
		Description: request.Description,
	})
	if err != nil {
		WriteError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, newCategoryResponse(*category))
}

// categoryResponse carries the public id and names the owner by handle; the
// internal id and the owner's user id are never returned.
type categoryResponse struct {
	ID          string    `json:"id"`
	Owner       string    `json:"owner"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func newCategoryResponse(category domain.Category) categoryResponse {
	return categoryResponse{
		ID:          category.PublicID,
		Owner:       category.OwnerHandle,
		Name:        category.Name,
		Description: category.Description,
		CreatedAt:   category.CreatedAt,
		UpdatedAt:   category.UpdatedAt,
	}
}

func newCategoryListResponse(categories []domain.Category) []categoryResponse {
	response := make([]categoryResponse, 0, len(categories))
	for _, category := range categories {
		response = append(response, newCategoryResponse(category))
	}
	return response
}

type LibraryHandler struct {
	libraries *usecase.LibraryUsecase
}

func NewLibraryHandler(libraries *usecase.LibraryUsecase) *LibraryHandler {
	return &LibraryHandler{libraries: libraries}
}

type createLibraryRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Visibility  string `json:"visibility"`
	Category    string `json:"category"`
}

// Create files a new library under the caller's category named in the body
// (POST /me/libraries), creating the category when needed.
func (h *LibraryHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	var request createLibraryRequest
	if err := decodeLibraryJSON(r, &request); err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_REQUEST_BODY", "request body must be valid JSON"))
		return
	}

	library, err := h.libraries.Create(r.Context(), actor, usecase.CreateLibraryInput{
		Name:        request.Name,
		Description: request.Description,
		Visibility:  request.Visibility,
		Category:    request.Category,
	})
	if err != nil {
		WriteError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, newLibraryResponse(*library))
}

func (h *LibraryHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, limitProvided, err := parseOptionalInt(r, "limit")
	if err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_LIMIT", "limit must be an integer"))
		return
	}
	offset, _, err := parseOptionalInt(r, "offset")
	if err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_OFFSET", "offset must be an integer"))
		return
	}
	actor, _ := auth.ActorFromContext(r.Context())

	libraries, err := h.libraries.List(r.Context(), actor, chi.URLParam(r, "category_id"), limit, limitProvided, offset)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	response := make([]libraryResponse, 0, len(libraries))
	for _, library := range libraries {
		response = append(response, newLibraryResponse(library))
	}
	writeJSON(w, http.StatusOK, response)
}

// ListAllPublic lists public libraries across every category.
func (h *LibraryHandler) ListAllPublic(w http.ResponseWriter, r *http.Request) {
	limit, limitProvided, err := parseOptionalInt(r, "limit")
	if err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_LIMIT", "limit must be an integer"))
		return
	}
	offset, _, err := parseOptionalInt(r, "offset")
	if err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_OFFSET", "offset must be an integer"))
		return
	}

	libraries, err := h.libraries.ListAllPublic(r.Context(), limit, limitProvided, offset)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	response := make([]libraryResponse, 0, len(libraries))
	for _, library := range libraries {
		response = append(response, newLibraryResponse(library))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *LibraryHandler) Get(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	library, err := h.libraries.Get(r.Context(), actor, chi.URLParam(r, "library_id"))
	if err != nil {
		WriteError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, newLibraryResponse(*library))
}

func (h *LibraryHandler) Update(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	var request updateLibraryRequest
	if err := decodeLibraryJSON(r, &request); err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_REQUEST_BODY", "request body must be valid JSON"))
		return
	}

	library, err := h.libraries.Update(r.Context(), actor, chi.URLParam(r, "library_id"), request.toUseCase())
	if err != nil {
		WriteError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, newLibraryResponse(*library))
}

// updateLibraryRequest accepts a partial PATCH body; nil fields keep their
// current value.
type updateLibraryRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Visibility  *string `json:"visibility"`
	Category    *string `json:"category"`
}

func (r updateLibraryRequest) toUseCase() usecase.UpdateLibraryInput {
	return usecase.UpdateLibraryInput{
		Name:        r.Name,
		Description: r.Description,
		Visibility:  r.Visibility,
		Category:    r.Category,
	}
}

type LibraryVersionHandler struct {
	versions *usecase.LibraryVersionUsecase
}

func NewLibraryVersionHandler(versions *usecase.LibraryVersionUsecase) *LibraryVersionHandler {
	return &LibraryVersionHandler{versions: versions}
}

func (h *LibraryVersionHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	var request struct {
		SourceCode string `json:"source_code"`
	}
	if err := decodeLibraryJSON(r, &request); err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_REQUEST_BODY", "request body must be valid JSON"))
		return
	}

	version, err := h.versions.Create(r.Context(), actor, chi.URLParam(r, "library_id"), request.SourceCode)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, newLibraryVersionResponse(*version))
}

func (h *LibraryVersionHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, limitProvided, err := parseOptionalInt(r, "limit")
	if err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_LIMIT", "limit must be an integer"))
		return
	}
	offset, _, err := parseOptionalInt(r, "offset")
	if err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_OFFSET", "offset must be an integer"))
		return
	}
	actor, _ := auth.ActorFromContext(r.Context())

	versions, err := h.versions.List(r.Context(), actor, chi.URLParam(r, "library_id"), limit, limitProvided, offset)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	response := make([]libraryVersionSummaryResponse, 0, len(versions))
	for _, version := range versions {
		response = append(response, newLibraryVersionSummaryResponse(version))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *LibraryVersionHandler) Get(w http.ResponseWriter, r *http.Request) {
	versionNumber, err := parseVersionNumber(r)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	actor, _ := auth.ActorFromContext(r.Context())

	version, err := h.versions.Get(r.Context(), actor, chi.URLParam(r, "library_id"), versionNumber)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, newLibraryVersionResponse(*version))
}

// maxVersionNumber is the largest version number the int32 sqlc query can
// represent; values above it would wrap (for example 4294967297 -> version 1).
const maxVersionNumber = 1<<31 - 1

// parseVersionNumber keeps the shared version contract. Non-numeric or
// non-positive values keep the exact spec §10 message. Values above
// maxVersionNumber get a distinct stable 400, because the sqlc query casts
// version_number to int32 and would otherwise alias an existing version.
func parseVersionNumber(r *http.Request) (int, error) {
	raw := chi.URLParam(r, "version_number")
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, apperror.InvalidInput("INVALID_VERSION_NUMBER", "version_number must be a positive integer")
	}
	if value > maxVersionNumber {
		return 0, apperror.InvalidInput("INVALID_VERSION_NUMBER", "version_number is too large")
	}
	return int(value), nil
}

// setSubmissionURLRequest is the PUT body; decodeJSON rejects unknown fields.
type setSubmissionURLRequest struct {
	SubmissionURL string `json:"submission_url"`
}

// SetSubmissionURL sets or replaces the owner's self-declared submission URL on
// one version and returns the version.
func (h *LibraryVersionHandler) SetSubmissionURL(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	var request setSubmissionURLRequest
	if err := decodeLibraryJSON(r, &request); err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_REQUEST_BODY", "request body must be valid JSON"))
		return
	}
	versionNumber, err := parseVersionNumber(r)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	version, err := h.versions.SetSubmissionURL(r.Context(), actor, chi.URLParam(r, "library_id"), versionNumber, request.SubmissionURL)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, newLibraryVersionResponse(*version))
}

// ClearSubmissionURL removes the self-declared submission URL from one version.
func (h *LibraryVersionHandler) ClearSubmissionURL(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	versionNumber, err := parseVersionNumber(r)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	if err := h.versions.ClearSubmissionURL(r.Context(), actor, chi.URLParam(r, "library_id"), versionNumber); err != nil {
		WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Both submission URL fields are null when the owner has not set a URL.
type libraryVersionResponse struct {
	ID                     string     `json:"id"`
	VersionNumber          int        `json:"version_number"`
	SourceCode             string     `json:"source_code"`
	ContentSHA256          string     `json:"content_sha256"`
	ByteSize               int        `json:"byte_size"`
	CreatedAt              time.Time  `json:"created_at"`
	SubmissionURL          *string    `json:"submission_url"`
	SubmissionURLUpdatedAt *time.Time `json:"submission_url_updated_at"`
}

func newLibraryVersionResponse(version domain.LibraryVersion) libraryVersionResponse {
	return libraryVersionResponse{
		ID:                     version.ID,
		VersionNumber:          version.VersionNumber,
		SourceCode:             version.SourceCode,
		ContentSHA256:          version.ContentSHA256,
		ByteSize:               version.ByteSize,
		CreatedAt:              version.CreatedAt,
		SubmissionURL:          optionalString(version.SubmissionURL),
		SubmissionURLUpdatedAt: version.SubmissionURLUpdatedAt,
	}
}

// libraryVersionSummaryResponse intentionally has no source_code field; list
// entries never carry the version body.
type libraryVersionSummaryResponse struct {
	ID                     string     `json:"id"`
	VersionNumber          int        `json:"version_number"`
	ContentSHA256          string     `json:"content_sha256"`
	ByteSize               int        `json:"byte_size"`
	CreatedAt              time.Time  `json:"created_at"`
	SubmissionURL          *string    `json:"submission_url"`
	SubmissionURLUpdatedAt *time.Time `json:"submission_url_updated_at"`
}

func newLibraryVersionSummaryResponse(version domain.LibraryVersion) libraryVersionSummaryResponse {
	return libraryVersionSummaryResponse{
		ID:                     version.ID,
		VersionNumber:          version.VersionNumber,
		ContentSHA256:          version.ContentSHA256,
		ByteSize:               version.ByteSize,
		CreatedAt:              version.CreatedAt,
		SubmissionURL:          optionalString(version.SubmissionURL),
		SubmissionURLUpdatedAt: version.SubmissionURLUpdatedAt,
	}
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// libraryResponse carries the public ids and names the owner by handle; no
// internal id or user id is returned.
type libraryResponse struct {
	ID          string                  `json:"id"`
	Owner       string                  `json:"owner"`
	Category    libraryCategoryResponse `json:"category"`
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	Visibility  string                  `json:"visibility"`
	CreatedAt   time.Time               `json:"created_at"`
	UpdatedAt   time.Time               `json:"updated_at"`
}

type libraryCategoryResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func newLibraryResponse(library domain.Library) libraryResponse {
	return libraryResponse{
		ID:          library.PublicID,
		Owner:       library.OwnerHandle,
		Category:    libraryCategoryResponse{ID: library.CategoryPublicID, Name: library.CategoryName},
		Name:        library.Name,
		Description: library.Description,
		Visibility:  string(library.Visibility),
		CreatedAt:   library.CreatedAt,
		UpdatedAt:   library.UpdatedAt,
	}
}
