package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Application struct {
	Id          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Name        string    `gorm:"uniqueIndex;not null"`
	Description string    `gorm:"type:text"`
	Labels      Labels    `gorm:"type:jsonb;default:'{}'"`
	CreatedBy   string    `gorm:"not null"`
	UpdatedBy   string    `gorm:"not null"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}
