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
	"github.com/tagokoro/tag-judge/backend/internal/librarycombine"
	"github.com/tagokoro/tag-judge/backend/internal/usecase"
)

// maxLibraryRequestBytes bounds the raw JSON body for library-sharing endpoints
// that accept a large source/adapter string. The usecases cap a decoded source
// at librarycombine.MaxSourceBytes bytes, but JSON escaping can expand one
// decoded byte to as many as 6 raw bytes (a \uXXXX escape), so a fully escaped
// valid body is still admitted while memory stays bounded. Submission and
// code-run endpoints keep decodeJSON's original unbounded contract.
const maxLibraryRequestBytes = 6*librarycombine.MaxSourceBytes + 64*1024

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

type NamespaceHandler struct {
	namespaces *usecase.NamespaceUsecase
}

func NewNamespaceHandler(namespaces *usecase.NamespaceUsecase) *NamespaceHandler {
	return &NamespaceHandler{namespaces: namespaces}
}

func (h *NamespaceHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.ActorFromContext(r.Context())
	if !ok {
		WriteError(w, r, apperror.Unauthorized("UNAUTHORIZED", "authentication required"))
		return
	}

	var request struct {
		Slug        string `json:"slug"`
		DisplayName string `json:"display_name"`
	}
	if err := decodeLibraryJSON(r, &request); err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_REQUEST_BODY", "request body must be valid JSON"))
		return
	}

	namespace, err := h.namespaces.Create(r.Context(), actor, request.Slug, request.DisplayName)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, newNamespaceResponse(*namespace))
}

func (h *NamespaceHandler) GetMine(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	namespace, err := h.namespaces.GetMine(r.Context(), actor)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, newNamespaceResponse(*namespace))
}

func (h *NamespaceHandler) GetBySlug(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "namespace_slug")
	namespace, err := h.namespaces.GetBySlug(r.Context(), slug)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, newNamespaceResponse(*namespace))
}

type namespaceResponse struct {
	ID          string    `json:"id"`
	Slug        string    `json:"slug"`
	DisplayName string    `json:"display_name"`
	OwnerUserID string    `json:"owner_user_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func newNamespaceResponse(namespace domain.Namespace) namespaceResponse {
	return namespaceResponse{
		ID:          namespace.ID,
		Slug:        namespace.Slug,
		DisplayName: namespace.DisplayName,
		OwnerUserID: namespace.OwnerUserID,
		CreatedAt:   namespace.CreatedAt,
		UpdatedAt:   namespace.UpdatedAt,
	}
}

type LibraryHandler struct {
	libraries *usecase.LibraryUsecase
}

func NewLibraryHandler(libraries *usecase.LibraryUsecase) *LibraryHandler {
	return &LibraryHandler{libraries: libraries}
}

type createLibraryRequest struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Visibility  string `json:"visibility"`
}

func (h *LibraryHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	var request createLibraryRequest
	if err := decodeLibraryJSON(r, &request); err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_REQUEST_BODY", "request body must be valid JSON"))
		return
	}

	library, err := h.libraries.Create(r.Context(), actor, chi.URLParam(r, "namespace_slug"), usecase.CreateLibraryInput{
		Slug:        request.Slug,
		Name:        request.Name,
		Description: request.Description,
		Visibility:  request.Visibility,
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

	libraries, err := h.libraries.List(r.Context(), actor, chi.URLParam(r, "namespace_slug"), limit, limitProvided, offset)
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
	library, err := h.libraries.Get(r.Context(), actor, chi.URLParam(r, "namespace_slug"), chi.URLParam(r, "library_slug"))
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

	library, err := h.libraries.Update(r.Context(), actor, chi.URLParam(r, "namespace_slug"), chi.URLParam(r, "library_slug"), request.toUseCase())
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
}

func (r updateLibraryRequest) toUseCase() usecase.UpdateLibraryInput {
	return usecase.UpdateLibraryInput{
		Name:        r.Name,
		Description: r.Description,
		Visibility:  r.Visibility,
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

	version, err := h.versions.Create(r.Context(), actor, chi.URLParam(r, "namespace_slug"), chi.URLParam(r, "library_slug"), request.SourceCode)
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

	versions, err := h.versions.List(r.Context(), actor, chi.URLParam(r, "namespace_slug"), chi.URLParam(r, "library_slug"), limit, limitProvided, offset)
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

	version, err := h.versions.Get(r.Context(), actor, chi.URLParam(r, "namespace_slug"), chi.URLParam(r, "library_slug"), versionNumber)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, newLibraryVersionResponse(*version))
}

// parseVersionNumber keeps the shared version contract: the path value must be
// a positive integer that fits int32. The library_version queries cast the
// number to int32, so a larger int64 value (for example 4294967297) would
// silently alias version 1 in GET and verify; reject it before the repository.
func parseVersionNumber(r *http.Request) (int, error) {
	raw := chi.URLParam(r, "version_number")
	value, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || value <= 0 {
		return 0, apperror.InvalidInput("INVALID_VERSION_NUMBER", "version_number must be a positive integer within int32 range")
	}
	return int(value), nil
}

type libraryVersionResponse struct {
	ID            string    `json:"id"`
	VersionNumber int       `json:"version_number"`
	SourceCode    string    `json:"source_code"`
	ContentSHA256 string    `json:"content_sha256"`
	ByteSize      int       `json:"byte_size"`
	CreatedAt     time.Time `json:"created_at"`
}

func newLibraryVersionResponse(version domain.LibraryVersion) libraryVersionResponse {
	return libraryVersionResponse{
		ID:            version.ID,
		VersionNumber: version.VersionNumber,
		SourceCode:    version.SourceCode,
		ContentSHA256: version.ContentSHA256,
		ByteSize:      version.ByteSize,
		CreatedAt:     version.CreatedAt,
	}
}

// libraryVersionSummaryResponse intentionally has no source_code field; list
// entries never carry the version body.
type libraryVersionSummaryResponse struct {
	ID            string    `json:"id"`
	VersionNumber int       `json:"version_number"`
	ContentSHA256 string    `json:"content_sha256"`
	ByteSize      int       `json:"byte_size"`
	CreatedAt     time.Time `json:"created_at"`
}

type LibraryVerificationHandler struct {
	verifications *usecase.LibraryVerificationUsecase
}

func NewLibraryVerificationHandler(v *usecase.LibraryVerificationUsecase) *LibraryVerificationHandler {
	return &LibraryVerificationHandler{verifications: v}
}

func (h *LibraryVerificationHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, _ := auth.ActorFromContext(r.Context())
	var req struct {
		ProblemID     int64  `json:"problem_id"`
		AdapterSource string `json:"adapter_source"`
	}
	if err := decodeLibraryJSON(r, &req); err != nil {
		WriteError(w, r, apperror.InvalidInput("INVALID_REQUEST_BODY", "request body must be valid JSON"))
		return
	}
	version, err := parseVersionNumber(r)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	v, err := h.verifications.Create(r.Context(), actor, chi.URLParam(r, "namespace_slug"), chi.URLParam(r, "library_slug"), int64(version), req.ProblemID, req.AdapterSource)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, newVerificationSummaryResponse(*v))
}

func parseVerificationID(r *http.Request) (int64, error) {
	v, err := strconv.ParseInt(chi.URLParam(r, "verification_id"), 10, 64)
	if err != nil {
		return 0, apperror.InvalidInput("INVALID_LIBRARY_VERIFICATION_ID", "verification_id must be an integer")
	}
	if v <= 0 {
		return 0, apperror.InvalidInput("INVALID_LIBRARY_VERIFICATION_ID", "verification_id must be a positive integer")
	}
	return v, nil
}

func (h *LibraryVerificationHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseVerificationID(r)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	actor, _ := auth.ActorFromContext(r.Context())
	v, cases, err := h.verifications.Get(r.Context(), actor, id)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, newVerificationDetailResponse(*v, cases))
}

func (h *LibraryVerificationHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, provided, err := parseOptionalInt(r, "limit")
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
	items, err := h.verifications.List(r.Context(), actor, limit, provided, offset)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	result := make([]verificationSummaryResponse, 0, len(items))
	for _, item := range items {
		result = append(result, newVerificationSummaryResponse(item))
	}
	writeJSON(w, http.StatusOK, result)
}

type verificationCaseResponse struct {
	TestCaseID      int64                   `json:"test_case_id"`
	Status          domain.SubmissionStatus `json:"status"`
	ExecutionTimeMS *int                    `json:"execution_time_ms"`
	MemoryKB        *int                    `json:"memory_kb"`
}
type verificationSummaryResponse struct {
	ID               int64                   `json:"id"`
	LibraryVersionID string                  `json:"library_version_id"`
	NamespaceSlug    string                  `json:"namespace_slug"`
	LibrarySlug      string                  `json:"library_slug"`
	VersionNumber    int                     `json:"version_number"`
	ProblemID        int64                   `json:"problem_id"`
	SubmissionID     int64                   `json:"submission_id"`
	Status           domain.SubmissionStatus `json:"status"`
	Score            int                     `json:"score"`
	AdapterSHA256    string                  `json:"adapter_sha256"`
	CreatedAt        time.Time               `json:"created_at"`
	JudgedAt         *time.Time              `json:"judged_at"`
}

type verificationDetailResponse struct {
	verificationSummaryResponse
	AdapterSource string                     `json:"adapter_source"`
	CaseResults   []verificationCaseResponse `json:"case_results"`
}

func newVerificationSummaryResponse(v domain.LibraryVerification) verificationSummaryResponse {
	return verificationSummaryResponse{ID: v.ID, LibraryVersionID: v.LibraryVersionID, NamespaceSlug: v.NamespaceSlug, LibrarySlug: v.LibrarySlug, VersionNumber: v.VersionNumber, ProblemID: v.ProblemID, SubmissionID: v.SubmissionID, Status: v.Status, Score: v.Score, AdapterSHA256: v.AdapterSHA256, CreatedAt: v.CreatedAt, JudgedAt: v.JudgedAt}
}

func newVerificationDetailResponse(v domain.LibraryVerification, cases []domain.SubmissionCaseResult) verificationDetailResponse {
	results := make([]verificationCaseResponse, 0, len(cases))
	for _, c := range cases {
		results = append(results, verificationCaseResponse{c.TestCaseID, c.Status, c.ExecutionTimeMS, c.MemoryKB})
	}
	return verificationDetailResponse{verificationSummaryResponse: newVerificationSummaryResponse(v), AdapterSource: v.AdapterSource, CaseResults: results}
}

func newLibraryVersionSummaryResponse(version domain.LibraryVersion) libraryVersionSummaryResponse {
	return libraryVersionSummaryResponse{
		ID:            version.ID,
		VersionNumber: version.VersionNumber,
		ContentSHA256: version.ContentSHA256,
		ByteSize:      version.ByteSize,
		CreatedAt:     version.CreatedAt,
	}
}

type libraryResponse struct {
	ID            string    `json:"id"`
	NamespaceSlug string    `json:"namespace_slug"`
	Slug          string    `json:"slug"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Visibility    string    `json:"visibility"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func newLibraryResponse(library domain.Library) libraryResponse {
	return libraryResponse{
		ID:            library.ID,
		NamespaceSlug: library.NamespaceSlug,
		Slug:          library.Slug,
		Name:          library.Name,
		Description:   library.Description,
		Visibility:    string(library.Visibility),
		CreatedAt:     library.CreatedAt,
		UpdatedAt:     library.UpdatedAt,
	}
}
