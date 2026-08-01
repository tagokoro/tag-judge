package usecase

import (
	"context"
	"encoding/hex"
	"strings"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

const (
	maxGroupNameLength        = 200
	maxGroupDescriptionLength = 2000
)

type GroupUsecase struct {
	groups repository.GroupRepository
}

func NewGroupUsecase(groups repository.GroupRepository) *GroupUsecase {
	return &GroupUsecase{groups: groups}
}

type CreateGroupInput struct {
	Name        string
	Description string
}

func (u *GroupUsecase) Create(ctx context.Context, actor auth.Actor, input CreateGroupInput) (*domain.Group, error) {
	if !actor.Role.CanCreateGroup() {
		return nil, apperror.Forbidden("GROUP_CREATE_FORBIDDEN", "author or admin role required")
	}

	name := strings.TrimSpace(input.Name)
	if name == "" || len([]rune(name)) > maxGroupNameLength {
		return nil, apperror.InvalidInput("INVALID_GROUP_NAME", "name must be between 1 and 200 characters")
	}
	if len([]rune(input.Description)) > maxGroupDescriptionLength {
		return nil, apperror.InvalidInput("INVALID_GROUP_DESCRIPTION", "description must be at most 2000 characters")
	}

	return u.groups.CreateWithOwner(ctx, repository.CreateGroupParams{
		Name:        name,
		Description: input.Description,
		CreatedBy:   actor.UserID,
	})
}

func (u *GroupUsecase) Get(ctx context.Context, actor auth.Actor, groupID int64) (*domain.Group, error) {
	if groupID <= 0 {
		return nil, apperror.InvalidInput("INVALID_GROUP_ID", "group_id must be a positive integer")
	}

	if actor.Role != domain.UserRoleAdmin {
		member, err := u.groups.FindMember(ctx, groupID, actor.UserID)
		if err != nil {
			return nil, err
		}
		if member == nil {
			if _, err := u.groups.FindByID(ctx, groupID); err != nil {
				return nil, err
			}
			return nil, apperror.Forbidden("GROUP_ACCESS_DENIED", "group membership required")
		}
	}

	return u.groups.FindByID(ctx, groupID)
}

func (u *GroupUsecase) List(ctx context.Context, actor auth.Actor) ([]domain.Group, error) {
	return u.groups.ListByUserID(ctx, actor.UserID)
}

type AddGroupMemberInput struct {
	UserID string
	Role   domain.GroupRole
}

func (u *GroupUsecase) AddMember(ctx context.Context, actor auth.Actor, groupID int64, input AddGroupMemberInput) (*domain.GroupMember, error) {
	if groupID <= 0 {
		return nil, apperror.InvalidInput("INVALID_GROUP_ID", "group_id must be a positive integer")
	}
	if !validUUID(input.UserID) {
		return nil, apperror.InvalidInput("INVALID_USER_ID", "user_id must be a UUID")
	}
	if !input.Role.Valid() {
		return nil, apperror.InvalidInput("INVALID_MEMBER_ROLE", "role must be member or owner")
	}

	if actor.Role != domain.UserRoleAdmin {
		member, err := u.groups.FindMember(ctx, groupID, actor.UserID)
		if err != nil {
			return nil, err
		}
		if member == nil {
			if _, err := u.groups.FindByID(ctx, groupID); err != nil {
				return nil, err
			}
			return nil, apperror.Forbidden("MEMBER_MANAGE_FORBIDDEN", "group owner or admin role required")
		}
		if member.Role != domain.GroupRoleOwner {
			return nil, apperror.Forbidden("MEMBER_MANAGE_FORBIDDEN", "group owner or admin role required")
		}
	}

	return u.groups.AddMember(ctx, repository.AddGroupMemberParams{
		GroupID: groupID,
		UserID:  input.UserID,
		Role:    input.Role,
	})
}

func validUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	compact := strings.ReplaceAll(value, "-", "")
	_, err := hex.DecodeString(compact)
	return err == nil
}
