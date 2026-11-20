package models

import (
	"time"
)

type Role struct {
	ID          string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	CompanyID   *string   `gorm:"type:uuid" json:"company_id"` // NULL for global role, non-null for tenant-specific role
	Name        string    `gorm:"not null;type:varchar(50)" json:"name"`
	Description *string   `gorm:"type:text" json:"description"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`

	RolePermissions []RolePermission `gorm:"foreignKey:RoleID" json:"role_permissions,omitempty"`
}

func (r *Role) TableName() string {
	return "roles"
}

func (r *Role) IsGlobal() bool {
	return r.CompanyID == nil
}
