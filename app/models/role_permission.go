package models

import (
	"time"
)

type RolePermission struct {
	ID              string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	RoleID          string    `gorm:"type:uuid;not null" json:"role_id"`
	FeatureActionID string    `gorm:"type:uuid;not null" json:"feature_action_id"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`

	Role          *Role          `gorm:"foreignKey:RoleID" json:"role,omitempty"`
	FeatureAction *FeatureAction `gorm:"foreignKey:FeatureActionID" json:"feature_action,omitempty"`
}

func (r *RolePermission) TableName() string {
	return "role_permissions"
}
