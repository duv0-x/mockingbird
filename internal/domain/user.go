package domain

type Role string

const (
	RoleReader   Role = "READER"
	RoleModifier Role = "MODIFIER"
	RoleAdmin    Role = "ADMIN"
)

func (r Role) CanRead() bool   { return r == RoleReader || r == RoleModifier || r == RoleAdmin }
func (r Role) CanModify() bool { return r == RoleModifier || r == RoleAdmin }
func (r Role) IsAdmin() bool   { return r == RoleAdmin }

type UserStatus string

const (
	UserActive   UserStatus = "ACTIVE"
	UserInactive UserStatus = "INACTIVE"
	UserLocked   UserStatus = "LOCKED"
)

// User mirrors com.castlemock.model.core.user.User. Password holds a bcrypt
// hash ($2a$/$2b$) in storage and is always "" in API responses.
type User struct {
	ID       string     `json:"id"`
	Username string     `json:"username"`
	Password string     `json:"password"`
	Email    string     `json:"email,omitempty"`
	FullName string     `json:"fullName,omitempty"`
	Updated  Time       `json:"updated"`
	Created  Time       `json:"created"`
	Status   UserStatus `json:"status"`
	Role     Role       `json:"role"`
}

// Redacted returns a copy safe to send over the API.
func (u User) Redacted() User {
	u.Password = ""
	return u
}
