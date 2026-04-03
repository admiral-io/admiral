package model

import (
	"database/sql/driver"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

type AuthSessionStatus string

const (
	AuthSessionStatusActive  AuthSessionStatus = "active"
	AuthSessionStatusRevoked AuthSessionStatus = "revoked"
)

func (s *AuthSessionStatus) Value() (driver.Value, error) {
	switch *s {
	case AuthSessionStatusActive, AuthSessionStatusRevoked:
		return string(*s), nil
	default:
		return nil, fmt.Errorf("invalid AuthSessionStatus value: %q", *s)
	}
}

func (s *AuthSessionStatus) Scan(value any) error {
	if value == nil {
		*s = ""
		return nil
	}

	switch v := value.(type) {
	case string:
		*s = AuthSessionStatus(v)
	case []byte:
		*s = AuthSessionStatus(v)
	default:
		return fmt.Errorf("cannot scan %T into AuthSessionStatus", value)
	}

	return nil
}

func (s *AuthSessionStatus) String() string {
	switch *s {
	case AuthSessionStatusActive, AuthSessionStatusRevoked:
		return string(*s)
	default:
		return ""
	}
}

type AuthSession struct {
	Id           uuid.UUID         `gorm:"column:id;primaryKey"`
	ParentID     *uuid.UUID        `gorm:"column:parent_id"`
	Subject      string            `gorm:"column:subject"`
	Issuer       string            `gorm:"column:issuer"`
	Status       AuthSessionStatus `gorm:"column:status;default:active"`
	AccessToken  []byte            `gorm:"column:access_token"`
	RefreshToken []byte            `gorm:"column:refresh_token"`
	IdToken      []byte            `gorm:"column:id_token"`
	CreatedAt    time.Time         `gorm:"column:created_at"`
	UpdatedAt    time.Time         `gorm:"column:updated_at"`
	ExpiresAt    *time.Time        `gorm:"column:expires_at"`
	DeletedAt    gorm.DeletedAt    `gorm:"column:deleted_at"`
}

func (AuthSession) TableName() string {
	return "auth_sessions"
}

func (s *AuthSession) ToOAuth2Token() *oauth2.Token {
	token := &oauth2.Token{
		AccessToken: string(s.AccessToken),
		TokenType:   "Bearer",
	}

	if s.ExpiresAt != nil {
		token.Expiry = *s.ExpiresAt
	}

	if len(s.RefreshToken) > 0 {
		token.RefreshToken = string(s.RefreshToken)
	}

	if len(s.IdToken) > 0 {
		token = token.WithExtra(map[string]any{"id_token": string(s.IdToken)})
	}

	return token
}
