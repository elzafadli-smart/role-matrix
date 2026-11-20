package models

import (
	"time"
)

type User struct {
	ID               string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	Name             string     `gorm:"not null;type:varchar(150)" json:"name"`
	DefaultCompanyID *string    `gorm:"type:uuid" json:"default_company_id"`
	CreatedAt        time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        *time.Time `json:"updated_at"`
	DeletedAt        *time.Time `json:"deleted_at"`

	UserCompanies []UserCompany `gorm:"foreignKey:UserID" json:"user_companies,omitempty"`
}

func (r *User) TableName() string {
	return "users"
}

type Company struct {
	ID           string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	OwnerID      string     `gorm:"type:uuid;not null" json:"owner_id"`
	Name         string     `gorm:"not null;type:varchar(60)" json:"name"`
	BusinessType string     `gorm:"not null;type:varchar(20)" json:"business_type"`
	CreatedAt    time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    *time.Time `json:"updated_at"`
	DeletedAt    *time.Time `json:"deleted_at"`

	BusinessUnits []BusinessUnit `gorm:"foreignKey:CompanyID" json:"business_units,omitempty"`
	Roles         []Role         `gorm:"foreignKey:CompanyID" json:"custom_roles,omitempty"`
}

func (r *Company) TableName() string {
	return "companies"
}

type BusinessUnit struct {
	ID        string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	CompanyID string     `gorm:"type:uuid;not null" json:"company_id"`
	Name      string     `gorm:"not null;type:varchar(60)" json:"name"`
	UnitType  string     `gorm:"not null;type:varchar(20)" json:"unit_type"` // cabang, outlet, pabrik
	Status    string     `gorm:"not null;default:'ACTIVE'" json:"status"`    // ACTIVE, INACTIVE
	CreatedAt time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at"`

	Company *Company `gorm:"foreignKey:CompanyID" json:"company,omitempty"`
}

func (r *BusinessUnit) TableName() string {
	return "business_units"
}

type UserCompany struct {
	ID                    string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID                string     `gorm:"type:uuid;not null" json:"user_id"`
	CompanyID             string     `gorm:"type:uuid;not null" json:"company_id"`
	RoleID                string     `gorm:"type:uuid;not null" json:"role_id"`
	CompanyAccess         bool       `gorm:"not null;default:true" json:"company_access"`
	UnitAccessMode        string     `gorm:"not null;default:'NONE'" json:"unit_access_mode"` // NONE, ALL, SELECTED
	AutoAssignFutureUnits bool       `gorm:"not null;default:false" json:"auto_assign_future_units"`
	JoinedAt              time.Time  `gorm:"not null;default:now()" json:"joined_at"`
	RevokedAt             *time.Time `json:"revoked_at"`

	User          *User              `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Company       *Company           `gorm:"foreignKey:CompanyID" json:"company,omitempty"`
	Role          *Role              `gorm:"foreignKey:RoleID" json:"role,omitempty"`
	BusinessUnits []UserBusinessUnit `gorm:"foreignKey:UserCompanyID" json:"assigned_units,omitempty"`
}

func (r *UserCompany) TableName() string {
	return "user_companies"
}

type UserBusinessUnit struct {
	ID             string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserCompanyID  string     `gorm:"type:uuid;not null" json:"user_company_id"`
	CompanyID      string     `gorm:"type:uuid;not null" json:"company_id"`
	UserID         string     `gorm:"type:uuid;not null" json:"user_id"`
	BusinessUnitID string     `gorm:"type:uuid;not null" json:"business_unit_id"`
	JoinedAt       time.Time  `gorm:"not null;default:now()" json:"joined_at"`
	RevokedAt      *time.Time `json:"revoked_at"`

	BusinessUnit *BusinessUnit `gorm:"foreignKey:BusinessUnitID" json:"business_unit,omitempty"`
}

func (r *UserBusinessUnit) TableName() string {
	return "user_business_units"
}

type Invitation struct {
	ID                    string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	CompanyID             string     `gorm:"type:uuid;not null" json:"company_id"`
	InviterID             string     `gorm:"type:uuid;not null" json:"inviter_id"`
	RoleID                string     `gorm:"type:uuid;not null" json:"role_id"`
	CompanyAccess         bool       `gorm:"not null;default:true" json:"company_access"`
	UnitAccessMode        string     `gorm:"not null;default:'NONE'" json:"unit_access_mode"` // NONE, ALL, SELECTED
	AutoAssignFutureUnits bool       `gorm:"not null;default:false" json:"auto_assign_future_units"`
	Email                 string     `gorm:"not null;type:varchar(254)" json:"email"`
	Token                 string     `gorm:"unique;not null;type:varchar(255)" json:"token"`
	Status                string     `gorm:"not null;default:'PENDING'" json:"status"` // PENDING, ACCEPTED, DECLINED, USER_REVOKED
	ExpiresAt             time.Time  `gorm:"not null" json:"expires_at"`
	AcceptedAt            *time.Time `json:"accepted_at"`
	DeclinedAt            *time.Time `json:"declined_at"`
	CreatedAt             time.Time  `gorm:"autoCreateTime" json:"created_at"`
}

func (r *Invitation) TableName() string {
	return "invitations"
}
