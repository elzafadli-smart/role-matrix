package models

import (
	"time"
)

type Feature struct {
	ID              string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	ModuleID        string    `gorm:"type:uuid;not null" json:"module_id"`
	Code            string    `gorm:"not null;type:varchar(50)" json:"code"`
	IsActionFeature bool      `gorm:"not null;default:false" json:"is_action_feature"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`

	Module         *Module         `gorm:"foreignKey:ModuleID" json:"module,omitempty"`
	FeatureActions []FeatureAction `gorm:"foreignKey:FeatureID" json:"feature_actions,omitempty"`
}

func (r *Feature) TableName() string {
	return "features"
}
