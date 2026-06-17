package model

type UserRole   string
type UserStatus string

const (
	RoleSuperAdmin  UserRole = "super_admin"
	RoleTenantAdmin UserRole = "tenant_admin"
	RoleUser        UserRole = "user"

	UserActive   UserStatus = "active"
	UserInactive UserStatus = "inactive"
)

type User struct {
	BaseModel
	TenantID     uint       `gorm:"index;not null" json:"tenant_id"`
	Username     string     `gorm:"size:64;not null" json:"username"`
	PasswordHash string     `gorm:"size:256;not null" json:"-"`
	DisplayName  string     `gorm:"size:64" json:"display_name"`
	AvatarURL    string     `gorm:"size:512" json:"avatar_url"`
	Role         UserRole   `gorm:"size:20;default:user" json:"role"`
	Status       UserStatus `gorm:"size:20;default:active" json:"status"`

	Tenant *Tenant `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
}

func (User) TableName() string { return "users" }

func (u *User) IsAdmin() bool {
	return u.Role == RoleSuperAdmin || u.Role == RoleTenantAdmin
}
