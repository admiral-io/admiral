package model

import (
	"time"

	"github.com/google/uuid"
)

type TerraformState struct {
	Id            uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	ComponentId   uuid.UUID `gorm:"type:uuid;not null"`
	EnvironmentId uuid.UUID `gorm:"type:uuid;not null"`
	Serial        int64     `gorm:"not null"`
	Lineage       string    `gorm:"type:text;not null;default:''"`
	StoragePath   string    `gorm:"type:text;not null"`
	ContentLength int64     `gorm:"not null;default:0"`
	ContentMD5    string    `gorm:"type:text;not null;default:''"`
	LockId        string    `gorm:"type:text;not null;default:''"`
	CreatedBy     string    `gorm:"type:text;not null"`
	CreatedAt     time.Time
}

type TerraformStateLock struct {
	ComponentId   uuid.UUID `gorm:"type:uuid;primaryKey"`
	EnvironmentId uuid.UUID `gorm:"type:uuid;primaryKey"`
	LockId        string    `gorm:"type:text;not null"`
	Operation     string    `gorm:"type:text;not null;default:''"`
	Who           string    `gorm:"type:text;not null;default:''"`
	Info          string    `gorm:"type:text;not null;default:''"`
	Version       string    `gorm:"type:text;not null;default:''"`
	Path          string    `gorm:"type:text;not null;default:''"`
	CreatedAt     time.Time
}
