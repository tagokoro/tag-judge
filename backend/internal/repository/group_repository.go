package repository

import (
	"context"

	"github.com/tagokoro/tag-judge/backend/internal/domain"
)

type CreateGroupParams struct {
	Name        string
	Description string
	CreatedBy   string
}

type AddGroupMemberParams struct {
	GroupID int64
	UserID  string
	Role    domain.GroupRole
}

type GroupRepository interface {
	CreateWithOwner(ctx context.Context, params CreateGroupParams) (*domain.Group, error)
	FindByID(ctx context.Context, id int64) (*domain.Group, error)
	ListByUserID(ctx context.Context, userID string) ([]domain.Group, error)
	FindMember(ctx context.Context, groupID int64, userID string) (*domain.GroupMember, error)
	AddMember(ctx context.Context, params AddGroupMemberParams) (*domain.GroupMember, error)
}
