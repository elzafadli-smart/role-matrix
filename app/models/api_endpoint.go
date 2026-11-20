package models

import (
	"time"
)

type ApiEndpoint struct {
	ID              string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	Method          string    `gorm:"not null;type:varchar(10)" json:"method"`
	Path            string    `gorm:"not null;type:varchar(255)" json:"path"`
	FeatureActionID string    `gorm:"type:uuid;not null" json:"feature_action_id"`
	ServiceName     string    `gorm:"not null;type:varchar(50)" json:"service_name"`
	Description     *string   `gorm:"type:text" json:"description"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`

	FeatureAction *FeatureAction `gorm:"foreignKey:FeatureActionID" json:"feature_action,omitempty"`
}

func (r *ApiEndpoint) TableName() string {
	return "api_endpoints"
}
