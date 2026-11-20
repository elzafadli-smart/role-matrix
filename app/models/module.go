package models

import (
	"time"
)

type Module struct {
	ID            string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	ApplicationID string    `gorm:"type:uuid;not null" json:"application_id"`
	Code          string    `gorm:"not null;type:varchar(50)" json:"code"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`

	Application *Application `gorm:"foreignKey:ApplicationID" json:"application,omitempty"`
	Features    []Feature    `gorm:"foreignKey:ModuleID" json:"features,omitempty"`
}

func (r *Module) TableName() string {
	return "modules"
}
