package models

import (
	"time"
)

type FeatureAction struct {
	ID        string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	FeatureID string    `gorm:"type:uuid;not null" json:"feature_id"`
	ActionID  string    `gorm:"type:uuid;not null" json:"action_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`

	Feature         *Feature         `gorm:"foreignKey:FeatureID" json:"feature,omitempty"`
	Action          *Action          `gorm:"foreignKey:ActionID" json:"action,omitempty"`
	RolePermissions []RolePermission `gorm:"foreignKey:FeatureActionID" json:"role_permissions,omitempty"`
	ApiEndpoints    []ApiEndpoint    `gorm:"foreignKey:FeatureActionID" json:"api_endpoints,omitempty"`
	UiRoutes        []UiRoute        `gorm:"foreignKey:FeatureActionID" json:"ui_routes,omitempty"`
}

func (r *FeatureAction) TableName() string {
	return "feature_actions"
}
