package model

import (
	"database/sql/driver"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

type AuthnToken struct {
	Id           uuid.UUID      `gorm:"column:id;primaryKey"`
	ParentID     *uuid.UUID     `gorm:"column:parent_id"`
	Subject      string         `gorm:"column:subject"`
	Issuer       string         `gorm:"column:issuer"`
	Kind         AuthnTokenKind `gorm:"column:kind"`
	AccessToken  []byte         `gorm:"column:access_token"`
	RefreshToken []byte         `gorm:"column:refresh_token"`
	IdToken      []byte         `gorm:"column:id_token"`
	CreatedAt    time.Time      `gorm:"column:created_at"`
	UpdatedAt    time.Time      `gorm:"column:updated_at"`
	ExpiresAt    time.Time      `gorm:"column:expires_at"`
}

func (AuthnToken) TableName() string {
	return "authn_tokens"
}

func (at *AuthnToken) ToOAuth2Token() *oauth2.Token {
	token := &oauth2.Token{
		AccessToken: string(at.AccessToken),
		Expiry:      at.ExpiresAt,
		TokenType:   "Bearer",
	}

	if len(at.RefreshToken) > 0 {
		token.RefreshToken = string(at.RefreshToken)
	}

	if len(at.IdToken) > 0 {
		token = token.WithExtra(map[string]interface{}{"id_token": string(at.IdToken)})
	}

	return token
}

type AuthnTokenKind string

const (
	AuthnTokenKindExternal AuthnTokenKind = "external"
	AuthnTokenKindUser     AuthnTokenKind = "user"
	AuthnTokenKindCluster  AuthnTokenKind = "cluster"
)

func (k *AuthnTokenKind) Value() (driver.Value, error) {
	switch *k {
	case AuthnTokenKindExternal, AuthnTokenKindUser, AuthnTokenKindCluster:
		return string(*k), nil
	default:
		return nil, fmt.Errorf("invalid AuthnTokenKind value")
	}
}

func (k *AuthnTokenKind) Scan(value interface{}) error {
	if value == nil {
		*k = ""
		return nil
	}

	switch v := value.(type) {
	case string:
		*k = AuthnTokenKind(v)
	case []byte:
		*k = AuthnTokenKind(v)
	default:
		return fmt.Errorf("cannot scan %T into AuthnTokenKind", value)
	}

	return nil
}

func (k *AuthnTokenKind) String() string {
	switch *k {
	case AuthnTokenKindExternal, AuthnTokenKindUser, AuthnTokenKindCluster:
		return string(*k)
	default:
		return ""
	}
}
