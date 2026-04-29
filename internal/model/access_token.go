package model

import (
	"database/sql/driver"
	"fmt"
	"time"

	"github.com/lib/pq"
	"golang.org/x/oauth2"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	commonv1 "go.admiral.io/sdk/proto/admiral/common/v1"
)

type AccessTokenKind string

const (
	AccessTokenKindPAT     AccessTokenKind = "pat"
	AccessTokenKindSAT     AccessTokenKind = "sat"
	AccessTokenKindSession AccessTokenKind = "session"
)

func (k *AccessTokenKind) Value() (driver.Value, error) {
	switch *k {
	case AccessTokenKindPAT, AccessTokenKindSAT, AccessTokenKindSession:
		return string(*k), nil
	default:
		return nil, fmt.Errorf("invalid AccessTokenKind value: %q", *k)
	}
}

func (k *AccessTokenKind) Scan(value any) error {
	if value == nil {
		*k = ""
		return nil
	}

	switch v := value.(type) {
	case string:
		*k = AccessTokenKind(v)
	case []byte:
		*k = AccessTokenKind(v)
	default:
		return fmt.Errorf("cannot scan %T into AccessTokenKind", value)
	}

	return nil
}

func (k *AccessTokenKind) String() string {
	switch *k {
	case AccessTokenKindPAT, AccessTokenKindSAT, AccessTokenKindSession:
		return string(*k)
	default:
		return ""
	}
}

type AccessTokenStatus string

const (
	AccessTokenStatusActive  AccessTokenStatus = "active"
	AccessTokenStatusRevoked AccessTokenStatus = "revoked"
)

func (s *AccessTokenStatus) Value() (driver.Value, error) {
	switch *s {
	case AccessTokenStatusActive, AccessTokenStatusRevoked:
		return string(*s), nil
	default:
		return nil, fmt.Errorf("invalid AccessTokenStatus value: %q", *s)
	}
}

func (s *AccessTokenStatus) Scan(value any) error {
	if value == nil {
		*s = ""
		return nil
	}

	switch v := value.(type) {
	case string:
		*s = AccessTokenStatus(v)
	case []byte:
		*s = AccessTokenStatus(v)
	default:
		return fmt.Errorf("cannot scan %T into AccessTokenStatus", value)
	}

	return nil
}

func (s *AccessTokenStatus) String() string {
	switch *s {
	case AccessTokenStatusActive, AccessTokenStatusRevoked:
		return string(*s)
	default:
		return ""
	}
}

type AccessTokenBindingType string

const (
	AccessTokenBindingTypeUser    AccessTokenBindingType = "user"
	AccessTokenBindingTypeCluster AccessTokenBindingType = "cluster"
	AccessTokenBindingTypeRunner  AccessTokenBindingType = "runner"
)

func (b *AccessTokenBindingType) Value() (driver.Value, error) {
	switch *b {
	case AccessTokenBindingTypeUser, AccessTokenBindingTypeCluster, AccessTokenBindingTypeRunner:
		return string(*b), nil
	default:
		return nil, fmt.Errorf("invalid AccessTokenBindingType value: %q", *b)
	}
}

func (b *AccessTokenBindingType) Scan(value any) error {
	if value == nil {
		*b = ""
		return nil
	}

	switch v := value.(type) {
	case string:
		*b = AccessTokenBindingType(v)
	case []byte:
		*b = AccessTokenBindingType(v)
	default:
		return fmt.Errorf("cannot scan %T into AccessTokenBindingType", value)
	}

	return nil
}

func (b *AccessTokenBindingType) String() string {
	switch *b {
	case AccessTokenBindingTypeUser, AccessTokenBindingTypeCluster, AccessTokenBindingTypeRunner:
		return string(*b)
	default:
		return ""
	}
}

type AccessToken struct {
	Id              string                 `gorm:"column:id;primaryKey"`
	Name            string                 `gorm:"column:name"`
	Subject         string                 `gorm:"column:subject"`
	Kind            AccessTokenKind        `gorm:"column:kind"`
	BindingType     AccessTokenBindingType `gorm:"column:binding_type"`
	Status          AccessTokenStatus      `gorm:"column:status;default:active"`
	TokenHash       []byte                 `gorm:"column:token_hash"`
	TokenPrefix     string                 `gorm:"column:token_prefix"`
	Scopes          pq.StringArray         `gorm:"column:scopes;type:text[]"`
	Issuer          string                 `gorm:"column:issuer"`
	IdpAccessToken  []byte                 `gorm:"column:idp_access_token"`
	IdpRefreshToken []byte                 `gorm:"column:idp_refresh_token"`
	IdpIdToken      []byte                 `gorm:"column:idp_id_token"`
	CreatedAt       time.Time              `gorm:"column:created_at"`
	UpdatedAt       time.Time              `gorm:"column:updated_at"`
	ExpiresAt       *time.Time             `gorm:"column:expires_at"`
	RevokedAt       *time.Time             `gorm:"column:revoked_at"`
	DeletedAt       gorm.DeletedAt         `gorm:"column:deleted_at"`
}

func (AccessToken) TableName() string {
	return "access_tokens"
}

func (at *AccessToken) ToProto() *commonv1.AccessToken {
	proto := &commonv1.AccessToken{
		Id:          at.Id,
		Name:        at.Name,
		TokenPrefix: at.TokenPrefix,
		Scopes:      at.Scopes,
		CreatedAt:   timestamppb.New(at.CreatedAt),
	}

	if at.ExpiresAt != nil {
		proto.ExpiresAt = timestamppb.New(*at.ExpiresAt)
	}

	if at.RevokedAt != nil {
		proto.RevokedAt = timestamppb.New(*at.RevokedAt)
	}

	switch at.Status {
	case AccessTokenStatusActive:
		proto.Status = commonv1.AccessTokenStatus_ACCESS_TOKEN_STATUS_ACTIVE
	case AccessTokenStatusRevoked:
		proto.Status = commonv1.AccessTokenStatus_ACCESS_TOKEN_STATUS_REVOKED
	}

	switch at.Kind {
	case AccessTokenKindPAT:
		proto.TokenType = commonv1.TokenType_TOKEN_TYPE_PAT
	case AccessTokenKindSAT:
		proto.TokenType = commonv1.TokenType_TOKEN_TYPE_SAT
	}

	switch at.BindingType {
	case AccessTokenBindingTypeUser:
		proto.BindingType = commonv1.BindingType_BINDING_TYPE_USER
	case AccessTokenBindingTypeCluster:
		proto.BindingType = commonv1.BindingType_BINDING_TYPE_CLUSTER
	case AccessTokenBindingTypeRunner:
		proto.BindingType = commonv1.BindingType_BINDING_TYPE_RUNNER
	}
	proto.BindingId = at.Subject

	return proto
}

func (at *AccessToken) IdPToken() *oauth2.Token {
	token := &oauth2.Token{
		TokenType: "Bearer",
	}

	if len(at.IdpAccessToken) > 0 {
		token.AccessToken = string(at.IdpAccessToken)
	}

	if at.ExpiresAt != nil {
		token.Expiry = *at.ExpiresAt
	}

	if len(at.IdpRefreshToken) > 0 {
		token.RefreshToken = string(at.IdpRefreshToken)
	}

	if len(at.IdpIdToken) > 0 {
		token = token.WithExtra(map[string]any{"id_token": string(at.IdpIdToken)})
	}

	return token
}
