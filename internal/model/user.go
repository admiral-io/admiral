package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserInfo struct {
	Email         string
	EmailVerified bool
	Name          string
	GivenName     string
	FamilyName    string
	PictureUrl    string
}

type User struct {
	Id              uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	ProviderSubject string
	Email           string
	EmailVerified   bool
	Name            string
	GivenName       string
	FamilyName      string
	PictureUrl      string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       gorm.DeletedAt `gorm:"index"`
}
