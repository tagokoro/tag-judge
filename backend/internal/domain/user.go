package domain

type UserRole string

const (
	UserRoleUser   UserRole = "user"
	UserRoleAuthor UserRole = "author"
	UserRoleAdmin  UserRole = "admin"
)
