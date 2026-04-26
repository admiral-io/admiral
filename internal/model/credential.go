package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	commonv1 "go.admiral.io/sdk/proto/admiral/common/v1"
	credentialv1 "go.admiral.io/sdk/proto/admiral/credential/v1"
)

const (
	AuthKindSSHKey      = "ssh_key"
	AuthKindBasicAuth   = "basic_auth"
	AuthKindBearerToken = "bearer_token"
)

const (
	CredentialTypeSSHKey      = "SSH_KEY"
	CredentialTypeBasicAuth   = "BASIC_AUTH"
	CredentialTypeBearerToken = "BEARER_TOKEN"
)

var credentialTypeToProto = map[string]credentialv1.CredentialType{
	CredentialTypeSSHKey:      credentialv1.CredentialType_CREDENTIAL_TYPE_SSH_KEY,
	CredentialTypeBasicAuth:   credentialv1.CredentialType_CREDENTIAL_TYPE_BASIC_AUTH,
	CredentialTypeBearerToken: credentialv1.CredentialType_CREDENTIAL_TYPE_BEARER_TOKEN,
}

var credentialTypeFromProto = map[credentialv1.CredentialType]string{
	credentialv1.CredentialType_CREDENTIAL_TYPE_SSH_KEY:      CredentialTypeSSHKey,
	credentialv1.CredentialType_CREDENTIAL_TYPE_BASIC_AUTH:   CredentialTypeBasicAuth,
	credentialv1.CredentialType_CREDENTIAL_TYPE_BEARER_TOKEN: CredentialTypeBearerToken,
}

func CredentialTypeFromProto(t credentialv1.CredentialType) string {
	return credentialTypeFromProto[t]
}

type SSHKeyAuth struct {
	PrivateKey string `json:"private_key"`
	Passphrase string `json:"passphrase,omitempty"`
}

type BasicAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type BearerTokenAuth struct {
	Token string `json:"token"`
}

// AuthConfig is the JSONB-backed polymorphic auth configuration.
// Kind is the discriminator; exactly one of the pointer fields is non-nil
// when populated.
//
// The raw field holds pre-serialized bytes (e.g. an encrypted envelope) that
// bypass normal JSON marshalling in Value(). This is set by the store layer
// before GORM writes to the database.
type AuthConfig struct {
	Kind        string           `json:"type"`
	SSHKey      *SSHKeyAuth      `json:"ssh_key,omitempty"`
	BasicAuth   *BasicAuth       `json:"basic_auth,omitempty"`
	BearerToken *BearerTokenAuth `json:"bearer_token,omitempty"`

	raw []byte `json:"-"` // encrypted envelope or nil
}

// SetRaw sets pre-serialized bytes (e.g. an encrypted envelope) that Value()
// will write directly to the database, bypassing normal JSON marshalling.
func (a *AuthConfig) SetRaw(b []byte) { a.raw = b }

// Raw returns the raw bytes from the last Scan, before any decryption.
func (a AuthConfig) Raw() []byte { return a.raw }

func (a AuthConfig) Value() (driver.Value, error) {
	// If raw bytes are set (encrypted envelope), pass them through directly.
	if a.raw != nil {
		return string(a.raw), nil
	}
	b, err := json.Marshal(a)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal auth config: %w", err)
	}
	return string(b), nil
}

func (a *AuthConfig) Scan(value any) error {
	if value == nil {
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case string:
		bytes = []byte(v)
	case []byte:
		bytes = v
	default:
		return fmt.Errorf("unsupported type for AuthConfig: %T", value)
	}
	// Stash raw bytes so the store layer can detect and decrypt envelopes.
	a.raw = make([]byte, len(bytes))
	copy(a.raw, bytes)
	return json.Unmarshal(bytes, a)
}

func (a AuthConfig) Validate(credType string) error {
	switch credType {
	case CredentialTypeSSHKey:
		if a.Kind != AuthKindSSHKey || a.SSHKey == nil {
			return fmt.Errorf("credential type %s requires ssh_key auth_config", credType)
		}
		if a.SSHKey.PrivateKey == "" {
			return fmt.Errorf("ssh_key credential requires a non-empty private_key")
		}
	case CredentialTypeBasicAuth:
		if a.Kind != AuthKindBasicAuth || a.BasicAuth == nil {
			return fmt.Errorf("credential type %s requires basic_auth auth_config", credType)
		}
		if a.BasicAuth.Username == "" || a.BasicAuth.Password == "" {
			return fmt.Errorf("basic_auth credential requires non-empty username and password")
		}
	case CredentialTypeBearerToken:
		if a.Kind != AuthKindBearerToken || a.BearerToken == nil {
			return fmt.Errorf("credential type %s requires bearer_token auth_config", credType)
		}
		if a.BearerToken.Token == "" {
			return fmt.Errorf("bearer_token credential requires a non-empty token")
		}
	case "":
		return fmt.Errorf("credential type is required")
	default:
		return fmt.Errorf("unsupported credential type: %s", credType)
	}
	return nil
}

func (a AuthConfig) ToProto() *credentialv1.AuthConfig {
	switch a.Kind {
	case AuthKindSSHKey:
		if a.SSHKey == nil {
			return nil
		}
		return &credentialv1.AuthConfig{
			Variant: &credentialv1.AuthConfig_SshKey{
				SshKey: &credentialv1.SSHKeyAuth{},
			},
		}
	case AuthKindBasicAuth:
		if a.BasicAuth == nil {
			return nil
		}
		return &credentialv1.AuthConfig{
			Variant: &credentialv1.AuthConfig_BasicAuth{
				BasicAuth: &credentialv1.BasicAuth{
					Username: a.BasicAuth.Username,
				},
			},
		}
	case AuthKindBearerToken:
		if a.BearerToken == nil {
			return nil
		}
		return &credentialv1.AuthConfig{
			Variant: &credentialv1.AuthConfig_BearerToken{
				BearerToken: &credentialv1.BearerTokenAuth{},
			},
		}
	}
	return nil
}

func AuthConfigFromProto(p *credentialv1.AuthConfig) AuthConfig {
	if p == nil {
		return AuthConfig{}
	}
	switch v := p.GetVariant().(type) {
	case nil:
		return AuthConfig{}
	case *credentialv1.AuthConfig_SshKey:
		ssh := v.SshKey
		return AuthConfig{
			Kind: AuthKindSSHKey,
			SSHKey: &SSHKeyAuth{
				PrivateKey: ssh.GetPrivateKey(),
				Passphrase: ssh.GetPassphrase(),
			},
		}
	case *credentialv1.AuthConfig_BasicAuth:
		basic := v.BasicAuth
		return AuthConfig{
			Kind: AuthKindBasicAuth,
			BasicAuth: &BasicAuth{
				Username: basic.GetUsername(),
				Password: basic.GetPassword(),
			},
		}
	case *credentialv1.AuthConfig_BearerToken:
		return AuthConfig{
			Kind:        AuthKindBearerToken,
			BearerToken: &BearerTokenAuth{Token: v.BearerToken.GetToken()},
		}
	default:
		panic(fmt.Sprintf("model: unmapped AuthConfig variant %T", v))
	}
}

type Credential struct {
	Id          uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Name        string     `gorm:"uniqueIndex;not null"`
	Description string     `gorm:"type:text"`
	Type        string     `gorm:"not null"`
	AuthConfig  AuthConfig `gorm:"type:jsonb;not null;default:'{}'"`
	Labels      Labels     `gorm:"type:jsonb;default:'{}'"`
	CreatedBy      string     `gorm:"not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      gorm.DeletedAt `gorm:"index"`
	CreatedByName  string         `gorm:"->;column:created_by_name"`
	CreatedByEmail string         `gorm:"->;column:created_by_email"`
}

func (c *Credential) ToProto() *credentialv1.Credential {
	out := &credentialv1.Credential{
		Id:          c.Id.String(),
		Name:        c.Name,
		Description: c.Description,
		Type:        credentialTypeToProto[c.Type],
		Labels:      map[string]string(c.Labels),
		CreatedBy:   &commonv1.ActorRef{Id: c.CreatedBy, DisplayName: c.CreatedByName, Email: c.CreatedByEmail},
		CreatedAt:   timestamppb.New(c.CreatedAt),
		UpdatedAt:   timestamppb.New(c.UpdatedAt),
	}
	out.AuthConfig = c.AuthConfig.ToProto()
	return out
}
