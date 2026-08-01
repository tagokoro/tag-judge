package domain

type UserRole string

const (
	UserRoleUser   UserRole = "user"
	UserRoleAuthor UserRole = "author"
	UserRoleAdmin  UserRole = "admin"
)

func (r UserRole) Valid() bool {
	switch r {
	case UserRoleUser, UserRoleAuthor, UserRoleAdmin:
		return true
	default:
		return false
	}
}

func (r UserRole) CanCreateProblem() bool {
	return r == UserRoleAuthor || r == UserRoleAdmin
}

func (r UserRole) CanCreateGroup() bool {
	return r == UserRoleAuthor || r == UserRoleAdmin
}
