package model

import (
	"database/sql/driver"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	commonv1 "buf.build/gen/go/admiral/common/protocolbuffers/go/admiral/common/v1"
)

type AuthnTokenKind string

const (
	AuthnTokenKindExternal AuthnTokenKind = "external"
	AuthnTokenKindUser     AuthnTokenKind = "user"
	AuthnTokenKindAgent    AuthnTokenKind = "agent"
)

func (k *AuthnTokenKind) Value() (driver.Value, error) {
	switch *k {
	case AuthnTokenKindExternal, AuthnTokenKindUser, AuthnTokenKindAgent:
		return string(*k), nil
	default:
		return nil, fmt.Errorf("invalid AuthnTokenKind value: %q", *k)
	}
}

func (k *AuthnTokenKind) Scan(value any) error {
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
	case AuthnTokenKindExternal, AuthnTokenKindUser, AuthnTokenKindAgent:
		return string(*k)
	default:
		return ""
	}
}

type AuthnTokenStatus string

const (
	AuthnTokenStatusActive   AuthnTokenStatus = "active"
	AuthnTokenStatusRevoked  AuthnTokenStatus = "revoked"
	AuthnTokenStatusRotating AuthnTokenStatus = "rotating"
)

func (s *AuthnTokenStatus) Value() (driver.Value, error) {
	switch *s {
	case AuthnTokenStatusActive, AuthnTokenStatusRevoked, AuthnTokenStatusRotating:
		return string(*s), nil
	default:
		return nil, fmt.Errorf("invalid AuthnTokenStatus value: %q", *s)
	}
}

func (s *AuthnTokenStatus) Scan(value any) error {
	if value == nil {
		*s = ""
		return nil
	}

	switch v := value.(type) {
	case string:
		*s = AuthnTokenStatus(v)
	case []byte:
		*s = AuthnTokenStatus(v)
	default:
		return fmt.Errorf("cannot scan %T into AuthnTokenStatus", value)
	}

	return nil
}

func (s *AuthnTokenStatus) String() string {
	switch *s {
	case AuthnTokenStatusActive, AuthnTokenStatusRevoked, AuthnTokenStatusRotating:
		return string(*s)
	default:
		return ""
	}
}

type AuthnToken struct {
	Id           uuid.UUID        `gorm:"column:id;primaryKey"`
	ParentID     *uuid.UUID       `gorm:"column:parent_id"`
	Name         string           `gorm:"column:name"`
	Subject      string           `gorm:"column:subject"`
	Issuer       string           `gorm:"column:issuer"`
	Kind         AuthnTokenKind   `gorm:"column:kind"`
	Status       AuthnTokenStatus `gorm:"column:status;default:active"`
	AccessToken  []byte           `gorm:"column:access_token"`
	RefreshToken []byte           `gorm:"column:refresh_token"`
	IdToken      []byte           `gorm:"column:id_token"`
	CreatedAt    time.Time        `gorm:"column:created_at"`
	UpdatedAt    time.Time        `gorm:"column:updated_at"`
	ExpiresAt    *time.Time       `gorm:"column:expires_at"`
	DeletedAt    gorm.DeletedAt   `gorm:"column:deleted_at"`
}

func (AuthnToken) TableName() string {
	return "authn_tokens"
}

func (at *AuthnToken) ToProto() *commonv1.AccessToken {
	proto := &commonv1.AccessToken{
		Id:        at.Id.String(),
		Name:      at.Name,
		CreatedAt: timestamppb.New(at.CreatedAt),
	}
	if at.ExpiresAt != nil {
		proto.ExpiresAt = timestamppb.New(*at.ExpiresAt)
	}

	switch at.Status {
	case AuthnTokenStatusActive:
		proto.Status = commonv1.AccessTokenStatus_ACCESS_TOKEN_STATUS_ACTIVE
	case AuthnTokenStatusRevoked:
		proto.Status = commonv1.AccessTokenStatus_ACCESS_TOKEN_STATUS_REVOKED
	case AuthnTokenStatusRotating:
		proto.Status = commonv1.AccessTokenStatus_ACCESS_TOKEN_STATUS_ROTATING
	}

	switch at.Kind {
	case AuthnTokenKindUser:
		proto.TokenType = commonv1.TokenType_TOKEN_TYPE_PAT
		proto.BindingType = commonv1.BindingType_BINDING_TYPE_USER
		proto.BindingId = at.Subject
	case AuthnTokenKindAgent:
		proto.TokenType = commonv1.TokenType_TOKEN_TYPE_AGT
		proto.BindingType = commonv1.BindingType_BINDING_TYPE_CLUSTER
		proto.BindingId = at.Subject
	}

	return proto
}

func (at *AuthnToken) ToOAuth2Token() *oauth2.Token {
	token := &oauth2.Token{
		AccessToken: string(at.AccessToken),
		TokenType:   "Bearer",
	}

	if at.ExpiresAt != nil {
		token.Expiry = *at.ExpiresAt
	}

	if len(at.RefreshToken) > 0 {
		token.RefreshToken = string(at.RefreshToken)
	}

	if len(at.IdToken) > 0 {
		token = token.WithExtra(map[string]any{"id_token": string(at.IdToken)})
	}

	return token
}
