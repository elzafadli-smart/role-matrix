package models

type Action struct {
	ID   string `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	Code string `gorm:"unique;not null;type:varchar(20)" json:"code"`
}

func (r *Action) TableName() string {
	return "actions"
}
