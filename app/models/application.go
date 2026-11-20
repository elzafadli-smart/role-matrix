package models

import (
	"time"
)

type Application struct {
	ID        string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	Code      string    `gorm:"unique;not null;type:varchar(50)" json:"code"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`

	Modules []Module `gorm:"foreignKey:ApplicationID" json:"modules,omitempty"`
}

func (r *Application) TableName() string {
	return "applications"
}
