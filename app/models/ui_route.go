package models

import (
	"time"
)

type UiRoute struct {
	ID              string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	FeatureActionID string    `gorm:"type:uuid;not null" json:"feature_action_id"`
	Permission      string    `gorm:"not null;type:varchar(100)" json:"permission"`
	Description     *string   `gorm:"type:text" json:"description"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`

	FeatureAction *FeatureAction `gorm:"foreignKey:FeatureActionID" json:"feature_action,omitempty"`
}

func (r *UiRoute) TableName() string {
	return "ui_routes"
}
