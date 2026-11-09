package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/infra/postgres/sqlc"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

// ContentSHA256 returns the lowercase hex SHA-256 of the raw UTF-8 bytes. It
// matches librarycombine.ContentSHA256 so the adapter_hash DB constraint and
// the API contract agree byte for byte.
func ContentSHA256(source string) string {
	sum := sha256.Sum256([]byte(source))
	return hex.EncodeToString(sum[:])
}

type NamespaceRepository struct {
	db      *pgxpool.Pool
	queries *sqlc.Queries
}

func NewNamespaceRepository(db *pgxpool.Pool) *NamespaceRepository {
	return &NamespaceRepository{db: db, queries: sqlc.New(db)}
}

func (r *NamespaceRepository) Create(ctx context.Context, params repository.CreateNamespaceParams) (*domain.Namespace, error) {
	row, err := r.queries.CreateNamespace(ctx, sqlc.CreateNamespaceParams{
		OwnerUserID: params.OwnerUserID,
		Slug:        params.Slug,
		DisplayName: params.DisplayName,
	})
	if err != nil {
		if isUniqueConstraintError(err, "namespaces_owner_user_unique") {
			return nil, apperror.Conflict("NAMESPACE_ALREADY_EXISTS", "user already has a namespace")
		}
		if isUniqueConstraintError(err, "namespaces_slug_unique") {
			return nil, apperror.Conflict("NAMESPACE_SLUG_TAKEN", "namespace slug is already taken")
		}
		return nil, err
	}
	return newNamespace(row.ID, row.OwnerUserID, row.Slug, row.DisplayName, row.CreatedAt, row.UpdatedAt), nil
}

func (r *NamespaceRepository) FindByOwner(ctx context.Context, ownerUserID string) (*domain.Namespace, error) {
	row, err := r.queries.GetNamespaceByOwner(ctx, ownerUserID)
	return namespaceFromQuery(ctx, row.ID, row.OwnerUserID, row.Slug, row.DisplayName, row.CreatedAt, row.UpdatedAt, err)
}

func (r *NamespaceRepository) FindBySlug(ctx context.Context, slug string) (*domain.Namespace, error) {
	row, err := r.queries.GetNamespaceBySlug(ctx, slug)
	return namespaceFromQuery(ctx, row.ID, row.OwnerUserID, row.Slug, row.DisplayName, row.CreatedAt, row.UpdatedAt, err)
}

func namespaceFromQuery(ctx context.Context, id pgtype.UUID, ownerUserID, slug, displayName string, createdAt, updatedAt pgtype.Timestamptz, err error) (*domain.Namespace, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("NAMESPACE_NOT_FOUND", "namespace not found")
	}
	if err != nil {
		return nil, err
	}
	return newNamespace(id, ownerUserID, slug, displayName, createdAt, updatedAt), nil
}

// isUniqueConstraintError reports whether err is the unique violation for the
// given constraint name. Mapping happens here and stays an adapter detail.
func isUniqueConstraintError(err error, constraintName string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505" && pgErr.ConstraintName == constraintName
}

func formatUUID(value pgtype.UUID) string {
	if !value.Valid {
		return ""
	}
	b := value.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

type LibraryRepository struct {
	db      *pgxpool.Pool
	queries *sqlc.Queries
}

func NewLibraryRepository(db *pgxpool.Pool) *LibraryRepository {
	return &LibraryRepository{db: db, queries: sqlc.New(db)}
}

func (r *LibraryRepository) Create(ctx context.Context, params repository.CreateLibraryParams) (*domain.Library, error) {
	row, err := r.queries.CreateLibrary(ctx, sqlc.CreateLibraryParams{
		NamespaceID: params.NamespaceID,
		Slug:        params.Slug,
		Name:        params.Name,
		Description: params.Description,
		Visibility:  string(params.Visibility),
	})
	if err != nil {
		if isUniqueConstraintError(err, "libraries_namespace_slug_unique") {
			return nil, apperror.Conflict("LIBRARY_SLUG_TAKEN", "library slug is already taken")
		}
		return nil, err
	}
	library := newLibrary(row.ID, row.NamespaceID, row.Slug, row.Name, row.Description, row.Visibility, row.CreatedAt, row.UpdatedAt)
	return &library, nil
}

func (r *LibraryRepository) Update(ctx context.Context, params repository.UpdateLibraryParams) (*domain.Library, error) {
	row, err := r.queries.UpdateLibrary(ctx, sqlc.UpdateLibraryParams{
		ID:          params.ID,
		Name:        nullableText(params.Name),
		Description: nullableText(params.Description),
		Visibility:  nullableText(params.Visibility),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("LIBRARY_NOT_FOUND", "library not found")
	}
	if err != nil {
		return nil, err
	}
	library := newLibrary(row.ID, row.NamespaceID, row.Slug, row.Name, row.Description, row.Visibility, row.CreatedAt, row.UpdatedAt)
	return &library, nil
}

func (r *LibraryRepository) FindBySlug(ctx context.Context, namespaceID, slug string) (*domain.Library, error) {
	row, err := r.queries.GetLibraryBySlug(ctx, sqlc.GetLibraryBySlugParams{
		NamespaceID: namespaceID,
		Slug:        slug,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("LIBRARY_NOT_FOUND", "library not found")
	}
	if err != nil {
		return nil, err
	}
	library := newLibrary(row.ID, row.NamespaceID, row.Slug, row.Name, row.Description, row.Visibility, row.CreatedAt, row.UpdatedAt)
	return &library, nil
}

func (r *LibraryRepository) ListPublic(ctx context.Context, namespaceID string, limit, offset int) ([]domain.Library, error) {
	rows, err := r.queries.ListPublicLibraries(ctx, sqlc.ListPublicLibrariesParams{
		NamespaceID:  namespaceID,
		ResultLimit:  int32(limit),
		ResultOffset: int32(offset),
	})
	return librariesFromRows(rows, err)
}

func (r *LibraryRepository) ListAll(ctx context.Context, namespaceID string, limit, offset int) ([]domain.Library, error) {
	rows, err := r.queries.ListNamespaceLibraries(ctx, sqlc.ListNamespaceLibrariesParams{
		NamespaceID:  namespaceID,
		ResultLimit:  int32(limit),
		ResultOffset: int32(offset),
	})
	return namespaceLibrariesFromRows(rows, err)
}

func librariesFromRows(rows []sqlc.ListPublicLibrariesRow, err error) ([]domain.Library, error) {
	if err != nil {
		return nil, err
	}
	libraries := make([]domain.Library, 0, len(rows))
	for _, row := range rows {
		libraries = append(libraries, newLibrary(row.ID, row.NamespaceID, row.Slug, row.Name, row.Description, row.Visibility, row.CreatedAt, row.UpdatedAt))
	}
	return libraries, nil
}

func namespaceLibrariesFromRows(rows []sqlc.ListNamespaceLibrariesRow, err error) ([]domain.Library, error) {
	if err != nil {
		return nil, err
	}
	libraries := make([]domain.Library, 0, len(rows))
	for _, row := range rows {
		libraries = append(libraries, newLibrary(row.ID, row.NamespaceID, row.Slug, row.Name, row.Description, row.Visibility, row.CreatedAt, row.UpdatedAt))
	}
	return libraries, nil
}

func nullableText(value *string) pgtype.Text {
	text := pgtype.Text{}
	if value != nil {
		text = pgtype.Text{String: *value, Valid: true}
	}
	return text
}

func newLibrary(id pgtype.UUID, namespaceID, slug, name, description, visibility string, createdAt, updatedAt pgtype.Timestamptz) domain.Library {
	return domain.Library{
		ID:          formatUUID(id),
		NamespaceID: namespaceID,
		Slug:        slug,
		Name:        name,
		Description: description,
		Visibility:  domain.LibraryVisibility(visibility),
		CreatedAt:   createdAt.Time,
		UpdatedAt:   updatedAt.Time,
	}
}

type LibraryVersionRepository struct {
	db      *pgxpool.Pool
	queries *sqlc.Queries
}

func NewLibraryVersionRepository(db *pgxpool.Pool) *LibraryVersionRepository {
	return &LibraryVersionRepository{db: db, queries: sqlc.New(db)}
}

func (r *LibraryVersionRepository) Create(ctx context.Context, params repository.CreateLibraryVersionParams) (*domain.LibraryVersion, error) {
	// The lock and the MAX-based insert are two statements in one transaction.
	// Keeping them separate makes the numbering read start with a fresh
	// snapshot after a concurrent creator releases the row lock.
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin library version transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := r.queries.WithTx(tx).LockLibraryByVersion(ctx, params.LibraryID); err != nil {
		return nil, err
	}
	row, err := r.queries.WithTx(tx).InsertLibraryVersion(ctx, sqlc.InsertLibraryVersionParams{
		LibraryID:     params.LibraryID,
		SourceCode:    params.SourceCode,
		ContentSha256: params.ContentSHA256,
		ByteSize:      int32(params.ByteSize),
		CreatedBy:     params.CreatedBy,
	})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit library version transaction: %w", err)
	}
	return &domain.LibraryVersion{
		ID:            row.ID,
		LibraryID:     row.LibraryID,
		VersionNumber: int(row.VersionNumber),
		SourceCode:    row.SourceCode,
		ContentSHA256: row.ContentSha256,
		ByteSize:      int(row.ByteSize),
		CreatedBy:     row.CreatedBy,
		CreatedAt:     row.CreatedAt.Time,
	}, nil
}

func (r *LibraryVersionRepository) FindByNumber(ctx context.Context, libraryID string, versionNumber int) (*domain.LibraryVersion, error) {
	row, err := r.queries.GetLibraryVersionByNumber(ctx, sqlc.GetLibraryVersionByNumberParams{
		LibraryID:     libraryID,
		VersionNumber: int32(versionNumber),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("LIBRARY_VERSION_NOT_FOUND", "library version not found")
	}
	if err != nil {
		return nil, err
	}
	return &domain.LibraryVersion{
		ID:            row.ID,
		LibraryID:     row.LibraryID,
		VersionNumber: int(row.VersionNumber),
		SourceCode:    row.SourceCode,
		ContentSHA256: row.ContentSha256,
		ByteSize:      int(row.ByteSize),
		CreatedBy:     row.CreatedBy,
		CreatedAt:     row.CreatedAt.Time,
	}, nil
}

func (r *LibraryVersionRepository) ListSummaries(ctx context.Context, params repository.ListLibraryVersionsParams) ([]domain.LibraryVersion, error) {
	rows, err := r.queries.ListLibraryVersionSummaries(ctx, sqlc.ListLibraryVersionSummariesParams{
		LibraryID:    params.LibraryID,
		ResultLimit:  int32(params.Limit),
		ResultOffset: int32(params.Offset),
	})
	if err != nil {
		return nil, err
	}
	versions := make([]domain.LibraryVersion, 0, len(rows))
	for _, row := range rows {
		versions = append(versions, domain.LibraryVersion{
			ID:            row.ID,
			LibraryID:     row.LibraryID,
			VersionNumber: int(row.VersionNumber),
			ContentSHA256: row.ContentSha256,
			ByteSize:      int(row.ByteSize),
			CreatedBy:     row.CreatedBy,
			CreatedAt:     row.CreatedAt.Time,
		})
	}
	return versions, nil
}

type LibraryVerificationRepository struct {
	db      *pgxpool.Pool
	queries *sqlc.Queries
}

func NewLibraryVerificationRepository(db *pgxpool.Pool) *LibraryVerificationRepository {
	return &LibraryVerificationRepository{db: db, queries: sqlc.New(db)}
}

// CreateWithSubmission commits the verification's submission row, existing
// outbox row, and verification row in one transaction, after re-checking the
// library visibility and version membership on a locked snapshot. When the
// re-check fails nothing is committed and the caller sees LIBRARY_NOT_FOUND.
func (r *LibraryVerificationRepository) CreateWithSubmission(ctx context.Context, params repository.CreateLibraryVerificationParams) (*domain.LibraryVerification, *domain.Submission, error) {
	if params.TimeLimitMS <= 0 {
		return nil, nil, fmt.Errorf("submission time limit must be positive")
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin verification transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := r.queries.WithTx(tx)

	// FOR SHARE locks the library row; a concurrent visibility PATCH cannot
	// commit between this read and our commit.
	locked, err := queries.LockLibraryForVerification(ctx, sqlc.LockLibraryForVerificationParams{
		NamespaceID: params.NamespaceID,
		LibraryID:   params.LibraryID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, apperror.NotFound("LIBRARY_NOT_FOUND", "library not found")
	}
	if err != nil {
		return nil, nil, fmt.Errorf("lock library for verification: %w", err)
	}
	if locked.Visibility != string(domain.LibraryVisibilityPublic) && formatUUID(locked.OwnerUserID) != params.ActorUserID {
		return nil, nil, apperror.NotFound("LIBRARY_NOT_FOUND", "library not found")
	}

	ownsVersion, err := queries.CountLibraryVersionInLibrary(ctx, sqlc.CountLibraryVersionInLibraryParams{
		LibraryVersionID: params.LibraryVersionID,
		LibraryID:        params.LibraryID,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("re-check version membership: %w", err)
	}
	if ownsVersion != 1 {
		return nil, nil, apperror.NotFound("LIBRARY_NOT_FOUND", "library not found")
	}

	row, err := queries.CreateSubmission(ctx, sqlc.CreateSubmissionParams{
		UserID:        params.ActorUserID,
		ProblemID:     params.ProblemID,
		Language:      string(params.Language),
		SourceCodeKey: params.SourceCodeKey,
		TestCasesKey:  params.TestCasesKey,
		Status:        string(domain.SubmissionStatusPending),
		Score:         0,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create verification submission: %w", err)
	}
	payload, err := json.Marshal(submissionOutboxPayload{
		SubmissionID:  fmt.Sprintf("%d", row.ID),
		Language:      params.Language,
		SourceCodeKey: params.SourceCodeKey,
		TestCasesKey:  params.TestCasesKey,
		TimeLimitMS:   params.TimeLimitMS,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("marshal verification outbox payload: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO submission_job_outbox (submission_id, payload) VALUES ($1, $2::jsonb)`, row.ID, payload); err != nil {
		return nil, nil, fmt.Errorf("create verification outbox record: %w", err)
	}
	verificationRow, err := queries.InsertLibraryVerification(ctx, sqlc.InsertLibraryVerificationParams{
		LibraryVersionID: params.LibraryVersionID,
		SubmissionID:     row.ID,
		AdapterSource:    params.AdapterSource,
		AdapterSha256:    ContentSHA256(params.AdapterSource),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create library verification record: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit verification transaction: %w", err)
	}

	submission := newSubmission(row.ID, row.UserID, row.ProblemID, row.Language, row.SourceCodeKey, row.TestCasesKey, row.Status, row.Score, row.CreatedAt, row.JudgedAt)
	verification := &domain.LibraryVerification{
		ID:               verificationRow.ID,
		LibraryVersionID: params.LibraryVersionID,
		VersionNumber:    0,
		ProblemID:        params.ProblemID,
		SubmissionID:     row.ID,
		Language:         params.Language,
		Status:           domain.SubmissionStatusPending,
		Score:            0,
		AdapterSource:    params.AdapterSource,
		AdapterSHA256:    ContentSHA256(params.AdapterSource),
		CreatedAt:        verificationRow.CreatedAt.Time,
	}
	return verification, &submission, nil
}

func (r *LibraryVerificationRepository) FindByID(ctx context.Context, id int64) (*domain.LibraryVerification, error) {
	row, err := r.queries.GetLibraryVerification(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("LIBRARY_VERIFICATION_NOT_FOUND", "library verification not found")
	}
	if err != nil {
		return nil, err
	}
	return verificationFromRow(row.ID, row.LibraryVersionID, row.SubmissionID, row.AdapterSource, row.AdapterSha256, row.CreatedAt, row.ProblemID, row.UserID, row.Status, row.Score, row.JudgedAt, row.VersionNumber, row.LibrarySlug, row.NamespaceSlug), nil
}

func (r *LibraryVerificationRepository) ListByUserID(ctx context.Context, params repository.ListLibraryVerificationsParams) ([]domain.LibraryVerification, error) {
	rows, err := r.queries.ListLibraryVerificationsByUser(ctx, sqlc.ListLibraryVerificationsByUserParams{
		UserID:       params.UserID,
		ResultLimit:  int32(params.Limit),
		ResultOffset: int32(params.Offset),
	})
	if err != nil {
		return nil, err
	}
	verifications := make([]domain.LibraryVerification, 0, len(rows))
	for _, row := range rows {
		verification := verificationFromRow(row.ID, row.LibraryVersionID, row.SubmissionID, "", row.AdapterSha256, row.CreatedAt, row.ProblemID, row.UserID, row.Status, row.Score, row.JudgedAt, row.VersionNumber, row.LibrarySlug, row.NamespaceSlug)
		verifications = append(verifications, *verification)
	}
	return verifications, nil
}

func verificationFromRow(id int64, libraryVersionID string, submissionID int64, adapterSource, adapterSHA256 string, createdAt pgtype.Timestamptz, problemID int64, userID, status string, score int32, judgedAt pgtype.Timestamptz, versionNumber int32, librarySlug, namespaceSlug string) *domain.LibraryVerification {
	var judgedAtTime *time.Time
	if judgedAt.Valid {
		value := judgedAt.Time
		judgedAtTime = &value
	}
	return &domain.LibraryVerification{
		ID:               id,
		LibraryVersionID: libraryVersionID,
		LibrarySlug:      librarySlug,
		NamespaceSlug:    namespaceSlug,
		VersionNumber:    int(versionNumber),
		ProblemID:        problemID,
		SubmissionID:     submissionID,
		UserID:           userID,
		Language:         domain.SubmissionLanguagePython,
		Status:           domain.SubmissionStatus(status),
		Score:            int(score),
		AdapterSource:    adapterSource,
		AdapterSHA256:    adapterSHA256,
		CreatedAt:        createdAt.Time,
		JudgedAt:         judgedAtTime,
	}
}

func newNamespace(id pgtype.UUID,
	ownerUserID string,
	slug string,
	displayName string,
	createdAt pgtype.Timestamptz,
	updatedAt pgtype.Timestamptz,
) *domain.Namespace {
	return &domain.Namespace{
		ID:          formatUUID(id),
		OwnerUserID: ownerUserID,
		Slug:        slug,
		DisplayName: displayName,
		CreatedAt:   createdAt.Time,
		UpdatedAt:   updatedAt.Time,
	}
}
