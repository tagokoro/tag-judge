package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/infra/postgres/sqlc"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

type GroupRepository struct {
	db      *pgxpool.Pool
	queries *sqlc.Queries
}

func NewGroupRepository(db *pgxpool.Pool) *GroupRepository {
	return &GroupRepository{db: db, queries: sqlc.New(db)}
}

func (r *GroupRepository) CreateWithOwner(ctx context.Context, params repository.CreateGroupParams) (*domain.Group, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	queries := r.queries.WithTx(tx)
	row, err := queries.CreateGroup(ctx, sqlc.CreateGroupParams{
		Name:        params.Name,
		Description: params.Description,
		CreatedBy:   params.CreatedBy,
	})
	if err != nil {
		return nil, err
	}

	_, err = queries.AddGroupMember(ctx, sqlc.AddGroupMemberParams{
		GroupID: row.ID,
		UserID:  params.CreatedBy,
		Role:    string(domain.GroupRoleOwner),
	})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	group := newGroup(row.ID, row.Name, row.Description, row.CreatedBy, row.CreatedAt, row.UpdatedAt)
	return &group, nil
}

func (r *GroupRepository) FindByID(ctx context.Context, id int64) (*domain.Group, error) {
	row, err := r.queries.GetGroup(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("GROUP_NOT_FOUND", "group not found")
	}
	if err != nil {
		return nil, err
	}

	group := newGroup(row.ID, row.Name, row.Description, row.CreatedBy, row.CreatedAt, row.UpdatedAt)
	return &group, nil
}

func (r *GroupRepository) ListByUserID(ctx context.Context, userID string) ([]domain.Group, error) {
	rows, err := r.queries.ListGroupsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	groups := make([]domain.Group, 0, len(rows))
	for _, row := range rows {
		groups = append(groups, newGroup(row.ID, row.Name, row.Description, row.CreatedBy, row.CreatedAt, row.UpdatedAt))
	}
	return groups, nil
}

func (r *GroupRepository) FindMember(ctx context.Context, groupID int64, userID string) (*domain.GroupMember, error) {
	row, err := r.queries.GetGroupMember(ctx, sqlc.GetGroupMemberParams{GroupID: groupID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	member := newGroupMember(row.GroupID, row.UserID, row.Role, row.CreatedAt)
	return &member, nil
}

func (r *GroupRepository) AddMember(ctx context.Context, params repository.AddGroupMemberParams) (*domain.GroupMember, error) {
	row, err := r.queries.AddGroupMember(ctx, sqlc.AddGroupMemberParams{
		GroupID: params.GroupID,
		UserID:  params.UserID,
		Role:    string(params.Role),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				return nil, apperror.Conflict("GROUP_MEMBER_EXISTS", "user is already a group member")
			case "23503":
				return nil, apperror.NotFound("USER_OR_GROUP_NOT_FOUND", "user or group not found")
			}
		}
		return nil, err
	}

	member := newGroupMember(row.GroupID, row.UserID, row.Role, row.CreatedAt)
	return &member, nil
}

func newGroup(id int64, name, description, createdBy string, createdAt, updatedAt pgtype.Timestamptz) domain.Group {
	return domain.Group{
		ID:          id,
		Name:        name,
		Description: description,
		CreatedBy:   createdBy,
		CreatedAt:   createdAt.Time,
		UpdatedAt:   updatedAt.Time,
	}
}

func newGroupMember(groupID int64, userID, role string, createdAt pgtype.Timestamptz) domain.GroupMember {
	return domain.GroupMember{
		GroupID:   groupID,
		UserID:    userID,
		Role:      domain.GroupRole(role),
		CreatedAt: createdAt.Time,
	}
}
