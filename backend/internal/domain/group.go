package domain

import "time"

type GroupRole string

const (
	GroupRoleMember GroupRole = "member"
	GroupRoleOwner  GroupRole = "owner"
)

func (r GroupRole) Valid() bool {
	return r == GroupRoleMember || r == GroupRoleOwner
}

type Group struct {
	ID          int64
	Name        string
	Description string
	CreatedBy   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type GroupMember struct {
	GroupID   int64
	UserID    string
	Role      GroupRole
	CreatedAt time.Time
}
