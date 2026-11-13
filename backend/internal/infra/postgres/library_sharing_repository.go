package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

// ContentSHA256 returns the lowercase hex SHA-256 of the raw UTF-8 bytes. The
// library_versions CHECK constraint recomputes the same digest with pgcrypto,
// so the value persisted here must agree byte for byte with PostgreSQL's
// encode(digest(...), 'hex').
func ContentSHA256(source string) string {
	sum := sha256.Sum256([]byte(source))
	return hex.EncodeToString(sum[:])
}

// CategoryRepository reads and edits categories. LibraryRepository creates
// and removes them, inside the transactions that file libraries.
type CategoryRepository struct {
	queries *sqlc.Queries
}

func NewCategoryRepository(db *pgxpool.Pool) *CategoryRepository {
	return &CategoryRepository{queries: sqlc.New(db)}
}

func (r *CategoryRepository) FindByPublicID(ctx context.Context, publicID string) (*domain.Category, error) {
	row, err := r.queries.GetCategoryByPublicID(ctx, publicID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, categoryNotFound()
	}
	if err != nil {
		return nil, err
	}
	return categoryFromRow(row), nil
}

func (r *CategoryRepository) ListByOwner(ctx context.Context, params repository.ListCategoriesParams) ([]domain.Category, error) {
	rows, err := r.queries.ListCategoriesByOwner(ctx, sqlc.ListCategoriesByOwnerParams{
		OwnerUserID:   params.OwnerUserID,
		IncludeHidden: params.IncludeHidden,
		ResultLimit:   int32(params.Limit),
		ResultOffset:  int32(params.Offset),
	})
	if err != nil {
		return nil, err
	}
	categories := make([]domain.Category, 0, len(rows))
	for _, row := range rows {
		categories = append(categories, *categoryFromRow(sqlc.GetCategoryByPublicIDRow(row)))
	}
	return categories, nil
}

// Update leaves OwnerHandle and HasPublicLibrary empty; the caller already
// resolved the category and copies them over.
func (r *CategoryRepository) Update(ctx context.Context, params repository.UpdateCategoryParams) (*domain.Category, error) {
	row, err := r.queries.UpdateCategory(ctx, sqlc.UpdateCategoryParams{
		ID:          params.ID,
		Name:        nullableText(params.Name),
		Description: nullableText(params.Description),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, categoryNotFound()
	}
	if err != nil {
		return nil, categoryWriteError(err)
	}
	return newCategory(row.ID, row.PublicID, row.OwnerUserID, row.Name, row.Description, row.CreatedAt, row.UpdatedAt), nil
}

// categoryWriteError maps the per-owner name index to its conflict.
func categoryWriteError(err error) error {
	if isUniqueConstraintError(err, "categories_owner_name_unique") {
		return apperror.Conflict("CATEGORY_NAME_TAKEN", "you already have a category with this name")
	}
	return err
}

func categoryNotFound() error {
	return apperror.NotFound("CATEGORY_NOT_FOUND", "category not found")
}

func categoryFromRow(row sqlc.GetCategoryByPublicIDRow) *domain.Category {
	category := newCategory(row.ID, row.PublicID, row.OwnerUserID, row.Name, row.Description, row.CreatedAt, row.UpdatedAt)
	category.OwnerHandle = row.OwnerHandle
	category.HasPublicLibrary = row.HasPublicLibrary
	return category
}

func newCategory(id, publicID, ownerUserID, name, description string, createdAt, updatedAt pgtype.Timestamptz) *domain.Category {
	return &domain.Category{
		ID:          id,
		PublicID:    publicID,
		OwnerUserID: ownerUserID,
		Name:        name,
		Description: description,
		CreatedAt:   createdAt.Time,
		UpdatedAt:   updatedAt.Time,
	}
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

// isForeignKeyConstraintError reports whether err is the foreign key violation
// for the given constraint name.
func isForeignKeyConstraintError(err error, constraintName string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23503" && pgErr.ConstraintName == constraintName
}

type LibraryRepository struct {
	db      *pgxpool.Pool
	queries *sqlc.Queries
}

func NewLibraryRepository(db *pgxpool.Pool) *LibraryRepository {
	return &LibraryRepository{db: db, queries: sqlc.New(db)}
}

func (r *LibraryRepository) Create(ctx context.Context, params repository.CreateLibraryParams) (*domain.Library, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin library transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := r.queries.WithTx(tx)

	categoryID, err := resolveCategory(ctx, queries, params.OwnerUserID, params.CategoryName)
	if err != nil {
		return nil, err
	}
	row, err := queries.CreateLibrary(ctx, sqlc.CreateLibraryParams{
		CategoryID:  categoryID,
		Name:        params.Name,
		Description: params.Description,
		Visibility:  string(params.Visibility),
	})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit library transaction: %w", err)
	}
	return libraryFromRow(sqlc.GetLibraryByPublicIDRow(row)), nil
}

// Update changes the library and, with CategoryName, moves it in the same
// transaction: the target category is found or created, and the category it
// leaves is removed once empty. Locks go profile first, then library, the same
// order Create takes.
func (r *LibraryRepository) Update(ctx context.Context, params repository.UpdateLibraryParams) (*domain.Library, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin library transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := r.queries.WithTx(tx)

	target := pgtype.Text{}
	var previous string
	if params.CategoryName != nil {
		categoryID, err := resolveCategory(ctx, queries, params.OwnerUserID, *params.CategoryName)
		if err != nil {
			return nil, err
		}
		target = pgtype.Text{String: categoryID, Valid: true}
		previous, err = queries.LockLibraryCategory(ctx, params.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, libraryNotFound()
		}
		if err != nil {
			return nil, err
		}
	}
	row, err := queries.UpdateLibrary(ctx, sqlc.UpdateLibraryParams{
		ID:          params.ID,
		Name:        nullableText(params.Name),
		Description: nullableText(params.Description),
		Visibility:  nullableText(params.Visibility),
		CategoryID:  target,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, libraryNotFound()
	}
	if err != nil {
		return nil, err
	}
	if target.Valid && previous != target.String {
		if err := queries.DeleteEmptyCategory(ctx, previous); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit library transaction: %w", err)
	}
	return libraryFromRow(sqlc.GetLibraryByPublicIDRow(row)), nil
}

// resolveCategory finds the owner's category by name (ignoring letter case)
// or creates it, inside the caller's transaction. Locking the owner's profile
// first serializes this per user, so two writers cannot both create the same
// category or pass the limit together.
func resolveCategory(ctx context.Context, queries *sqlc.Queries, ownerUserID, name string) (string, error) {
	if _, err := queries.LockUserProfileForCategory(ctx, ownerUserID); errors.Is(err, pgx.ErrNoRows) {
		return "", apperror.Conflict("HANDLE_REQUIRED", "create a handle before creating a library")
	} else if err != nil {
		return "", err
	}
	id, err := queries.FindCategoryIDByName(ctx, sqlc.FindCategoryIDByNameParams{OwnerUserID: ownerUserID, Name: name})
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	count, err := queries.CountCategoriesByOwner(ctx, ownerUserID)
	if err != nil {
		return "", err
	}
	if count >= domain.MaxCategoriesPerUser {
		return "", apperror.Conflict("CATEGORY_LIMIT", fmt.Sprintf("a user can own at most %d categories", domain.MaxCategoriesPerUser))
	}
	id, err = queries.CreateCategory(ctx, sqlc.CreateCategoryParams{OwnerUserID: ownerUserID, Name: name})
	if err != nil {
		return "", categoryWriteError(err)
	}
	return id, nil
}

func libraryNotFound() error {
	return apperror.NotFound("LIBRARY_NOT_FOUND", "library not found")
}

func (r *LibraryRepository) FindByPublicID(ctx context.Context, publicID string) (*domain.Library, error) {
	row, err := r.queries.GetLibraryByPublicID(ctx, publicID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, libraryNotFound()
	}
	if err != nil {
		return nil, err
	}
	return libraryFromRow(row), nil
}

func (r *LibraryRepository) ListByCategory(ctx context.Context, params repository.ListLibrariesParams) ([]domain.Library, error) {
	rows, err := r.queries.ListCategoryLibraries(ctx, sqlc.ListCategoryLibrariesParams{
		CategoryID:     params.CategoryID,
		IncludePrivate: params.IncludePrivate,
		ResultLimit:    int32(params.Limit),
		ResultOffset:   int32(params.Offset),
	})
	if err != nil {
		return nil, err
	}
	libraries := make([]domain.Library, 0, len(rows))
	for _, row := range rows {
		libraries = append(libraries, *libraryFromRow(sqlc.GetLibraryByPublicIDRow(row)))
	}
	return libraries, nil
}

func (r *LibraryRepository) ListAllPublic(ctx context.Context, limit, offset int) ([]domain.Library, error) {
	rows, err := r.queries.ListAllPublicLibraries(ctx, sqlc.ListAllPublicLibrariesParams{
		ResultLimit:  int32(limit),
		ResultOffset: int32(offset),
	})
	if err != nil {
		return nil, err
	}
	libraries := make([]domain.Library, 0, len(rows))
	for _, row := range rows {
		libraries = append(libraries, *libraryFromRow(sqlc.GetLibraryByPublicIDRow(row)))
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

// libraryFromRow maps the shared library row. Every library query selects the
// same columns, so their generated row structs convert to this one.
func libraryFromRow(row sqlc.GetLibraryByPublicIDRow) *domain.Library {
	return &domain.Library{
		ID:               row.ID,
		PublicID:         row.PublicID,
		CategoryPublicID: row.CategoryPublicID,
		CategoryName:     row.CategoryName,
		OwnerUserID:      row.OwnerUserID,
		OwnerHandle:      row.OwnerHandle,
		Name:             row.Name,
		Description:      row.Description,
		Visibility:       domain.LibraryVisibility(row.Visibility),
		CreatedAt:        row.CreatedAt.Time,
		UpdatedAt:        row.UpdatedAt.Time,
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
	// snapshot after a concurrent creator releases the row lock. The lock is
	// an UPDATE that also bumps the library's updated_at.
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin library version transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := r.queries.WithTx(tx).TouchLibraryForVersion(ctx, params.LibraryID); errors.Is(err, pgx.ErrNoRows) {
		return nil, libraryNotFound()
	} else if err != nil {
		return nil, err
	}
	row, err := r.queries.WithTx(tx).InsertLibraryVersion(ctx, sqlc.InsertLibraryVersionParams{
		LibraryID:     params.LibraryID,
		SourceCode:    params.SourceCode,
		ContentSha256: ContentSHA256(params.SourceCode),
		ByteSize:      int32(len([]byte(params.SourceCode))),
		CreatedBy:     params.CreatedBy,
	})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit library version transaction: %w", err)
	}
	return libraryVersionFromRow(sqlc.GetLibraryVersionByNumberRow(row)), nil
}

func (r *LibraryVersionRepository) FindByNumber(ctx context.Context, libraryID string, versionNumber int, actorUserID string) (*domain.LibraryVersion, error) {
	row, err := r.queries.GetLibraryVersionByNumber(ctx, sqlc.GetLibraryVersionByNumberParams{
		LibraryID:     libraryID,
		VersionNumber: int32(versionNumber),
		ActorUserID:   actorUserID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("LIBRARY_VERSION_NOT_FOUND", "library version not found")
	}
	if err != nil {
		return nil, err
	}
	return libraryVersionFromRow(row), nil
}

func (r *LibraryVersionRepository) ListSummaries(ctx context.Context, params repository.ListLibraryVersionsParams) ([]domain.LibraryVersion, error) {
	rows, err := r.queries.ListLibraryVersionSummaries(ctx, sqlc.ListLibraryVersionSummariesParams{
		LibraryID:    params.LibraryID,
		ResultLimit:  int32(params.Limit),
		ResultOffset: int32(params.Offset),
		ActorUserID:  params.ActorUserID,
	})
	if err != nil {
		return nil, err
	}
	versions := make([]domain.LibraryVersion, 0, len(rows))
	for _, row := range rows {
		versions = append(versions, domain.LibraryVersion{
			ID:                     row.ID,
			LibraryID:              row.LibraryID,
			VersionNumber:          int(row.VersionNumber),
			ContentSHA256:          row.ContentSha256,
			ByteSize:               int(row.ByteSize),
			CreatedBy:              row.CreatedBy,
			CreatedAt:              row.CreatedAt.Time,
			SubmissionURL:          row.SubmissionUrl.String,
			SubmissionURLUpdatedAt: optionalTime(row.SubmissionUrlUpdatedAt),
		})
	}
	return versions, nil
}

// SetSubmissionURL sets or clears the self-declared submission URL. A non-nil
// URL first passes the authoritative domain validator (so invalid URLs are
// rejected even when called outside the API layer) and is stored normalized.
// The UPDATE itself checks that the actor owns the library, so a non-owner or
// a missing version returns LIBRARY_VERSION_NOT_FOUND without writing.
func (r *LibraryVersionRepository) SetSubmissionURL(ctx context.Context, params repository.SetLibraryVersionSubmissionURLParams) (*domain.LibraryVersion, error) {
	submissionURL := pgtype.Text{}
	if params.SubmissionURL != nil {
		normalized, err := domain.ValidateSubmissionURL(*params.SubmissionURL)
		if err != nil {
			return nil, apperror.InvalidInput("INVALID_SUBMISSION_URL", "submission_url must be an absolute HTTP(S) URL with a host, at most 2048 bytes, and no control characters")
		}
		submissionURL = pgtype.Text{String: normalized, Valid: true}
	}
	row, err := r.queries.SetLibraryVersionSubmissionURL(ctx, sqlc.SetLibraryVersionSubmissionURLParams{
		SubmissionUrl: submissionURL,
		LibraryID:     params.LibraryID,
		VersionNumber: int32(params.VersionNumber),
		ActorUserID:   params.ActorUserID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("LIBRARY_VERSION_NOT_FOUND", "library version not found")
	}
	if err != nil {
		return nil, err
	}
	return libraryVersionFromRow(sqlc.GetLibraryVersionByNumberRow(row)), nil
}

// libraryVersionFromRow maps a full version row. Insert, Get, and Set select
// identical columns, so their generated row structs convert to this one.
func libraryVersionFromRow(row sqlc.GetLibraryVersionByNumberRow) *domain.LibraryVersion {
	return &domain.LibraryVersion{
		ID:                     row.ID,
		LibraryID:              row.LibraryID,
		VersionNumber:          int(row.VersionNumber),
		SourceCode:             row.SourceCode,
		ContentSHA256:          row.ContentSha256,
		ByteSize:               int(row.ByteSize),
		CreatedBy:              row.CreatedBy,
		CreatedAt:              row.CreatedAt.Time,
		SubmissionURL:          row.SubmissionUrl.String,
		SubmissionURLUpdatedAt: optionalTime(row.SubmissionUrlUpdatedAt),
	}
}

func optionalTime(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time
	return &t
}
