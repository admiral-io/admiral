package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	commonv1 "go.admiral.io/sdk/proto/admiral/common/v1"
	sourcev1 "go.admiral.io/sdk/proto/admiral/source/v1"
)

const (
	SourceConfigKindTerraform = "TERRAFORM"
	SourceConfigKindHelm      = "HELM"
)

const (
	SourceTypeGit       = "GIT"
	SourceTypeTerraform = "TERRAFORM"
	SourceTypeHelm      = "HELM"
	SourceTypeOCI       = "OCI"
	SourceTypeHTTP      = "HTTP"
)

const (
	SourceTestStatusSuccess = "SUCCESS"
	SourceTestStatusFailure = "FAILURE"
)

var sourceTypeToProto = map[string]sourcev1.SourceType{
	SourceTypeGit:       sourcev1.SourceType_SOURCE_TYPE_GIT,
	SourceTypeTerraform: sourcev1.SourceType_SOURCE_TYPE_TERRAFORM,
	SourceTypeHelm:      sourcev1.SourceType_SOURCE_TYPE_HELM,
	SourceTypeOCI:       sourcev1.SourceType_SOURCE_TYPE_OCI,
	SourceTypeHTTP:      sourcev1.SourceType_SOURCE_TYPE_HTTP,
}

var sourceTypeFromProto = map[sourcev1.SourceType]string{
	sourcev1.SourceType_SOURCE_TYPE_GIT:       SourceTypeGit,
	sourcev1.SourceType_SOURCE_TYPE_TERRAFORM: SourceTypeTerraform,
	sourcev1.SourceType_SOURCE_TYPE_HELM:      SourceTypeHelm,
	sourcev1.SourceType_SOURCE_TYPE_OCI:       SourceTypeOCI,
	sourcev1.SourceType_SOURCE_TYPE_HTTP:      SourceTypeHTTP,
}

var sourceTestStatusToProto = map[string]sourcev1.SourceTestStatus{
	SourceTestStatusSuccess: sourcev1.SourceTestStatus_SOURCE_TEST_STATUS_SUCCESS,
	SourceTestStatusFailure: sourcev1.SourceTestStatus_SOURCE_TEST_STATUS_FAILURE,
}

// SourceTypeFromProto returns the DB string for a proto SourceType, or "" if
// the enum is unrecognized (callers Validate will surface the error).
func SourceTypeFromProto(t sourcev1.SourceType) string {
	return sourceTypeFromProto[t]
}

type TerraformConfig struct {
	Namespace  string `json:"namespace"`
	ModuleName string `json:"module_name"`
	System     string `json:"system"`
}

type HelmConfig struct {
	ChartName string `json:"chart_name"`
}

// SourceConfig is the JSONB-backed polymorphic per-type configuration.
// Kind is the discriminator; exactly one of the pointer fields is non-nil
// when populated. Source types whose location is fully expressed by `url`
// (GIT, OCI, HTTP) leave this empty.
type SourceConfig struct {
	Kind      string           `json:"type,omitempty"`
	Terraform *TerraformConfig `json:"terraform,omitempty"`
	Helm      *HelmConfig      `json:"helm,omitempty"`
}

func (c SourceConfig) Value() (driver.Value, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal source config: %w", err)
	}
	return string(b), nil
}

func (c *SourceConfig) Scan(value any) error {
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
		return fmt.Errorf("unsupported type for SourceConfig: %T", value)
	}
	return json.Unmarshal(bytes, c)
}

func (c SourceConfig) Validate(sourceType string) error {
	switch sourceType {
	case SourceTypeGit, SourceTypeOCI, SourceTypeHTTP:
		if c.Kind != "" || c.Terraform != nil || c.Helm != nil {
			return fmt.Errorf("source type %s must not carry a source_config", sourceType)
		}
	case SourceTypeTerraform:
		if c.Kind != SourceConfigKindTerraform || c.Terraform == nil {
			return fmt.Errorf("source type %s requires terraform source_config", sourceType)
		}
		if c.Terraform.Namespace == "" || c.Terraform.ModuleName == "" || c.Terraform.System == "" {
			return fmt.Errorf("terraform source_config requires non-empty namespace, module_name, and system")
		}
	case SourceTypeHelm:
		if c.Kind != SourceConfigKindHelm || c.Helm == nil {
			return fmt.Errorf("source type %s requires helm source_config", sourceType)
		}
		if c.Helm.ChartName == "" {
			return fmt.Errorf("helm source_config requires a non-empty chart_name")
		}
	case "":
		return fmt.Errorf("source type is required")
	default:
		return fmt.Errorf("unsupported source type: %s", sourceType)
	}
	return nil
}

func (c SourceConfig) ToProto() *sourcev1.SourceConfig {
	switch c.Kind {
	case SourceConfigKindTerraform:
		if c.Terraform == nil {
			return nil
		}
		return &sourcev1.SourceConfig{
			Variant: &sourcev1.SourceConfig_Terraform{
				Terraform: &sourcev1.TerraformConfig{
					Namespace:  c.Terraform.Namespace,
					ModuleName: c.Terraform.ModuleName,
					System:     c.Terraform.System,
				},
			},
		}
	case SourceConfigKindHelm:
		if c.Helm == nil {
			return nil
		}
		return &sourcev1.SourceConfig{
			Variant: &sourcev1.SourceConfig_Helm{
				Helm: &sourcev1.HelmConfig{ChartName: c.Helm.ChartName},
			},
		}
	}
	return nil
}

func SourceConfigFromProto(p *sourcev1.SourceConfig) SourceConfig {
	if p == nil {
		return SourceConfig{}
	}
	switch v := p.GetVariant().(type) {
	case nil:
		return SourceConfig{}
	case *sourcev1.SourceConfig_Terraform:
		tf := v.Terraform
		return SourceConfig{
			Kind: SourceConfigKindTerraform,
			Terraform: &TerraformConfig{
				Namespace:  tf.GetNamespace(),
				ModuleName: tf.GetModuleName(),
				System:     tf.GetSystem(),
			},
		}
	case *sourcev1.SourceConfig_Helm:
		return SourceConfig{
			Kind: SourceConfigKindHelm,
			Helm: &HelmConfig{ChartName: v.Helm.GetChartName()},
		}
	default:
		panic(fmt.Sprintf("model: unmapped SourceConfig variant %T", v))
	}
}

// credentialSourceCompat defines which credential types are accepted by each
// source type. For GIT sources, URL scheme further restricts the match (see
// ValidateCredentialSourceCompat).
var credentialSourceCompat = map[string]map[string]bool{
	SourceTypeGit:       {CredentialTypeSSHKey: true, CredentialTypeBasicAuth: true},
	SourceTypeTerraform: {CredentialTypeBearerToken: true},
	SourceTypeHelm:      {CredentialTypeBasicAuth: true, CredentialTypeBearerToken: true},
	SourceTypeOCI:       {CredentialTypeBasicAuth: true, CredentialTypeBearerToken: true},
	SourceTypeHTTP:      {CredentialTypeBasicAuth: true, CredentialTypeBearerToken: true},
}

// ValidateCredentialSourceCompat checks that a credential type is compatible
// with a source type and URL. For GIT sources, SSH_KEY requires an SSH URL and
// BASIC_AUTH requires an HTTPS URL. For all other source types, SSH_KEY is
// never valid.
func ValidateCredentialSourceCompat(credType, sourceType, url string) error {
	compat, ok := credentialSourceCompat[sourceType]
	if !ok {
		return fmt.Errorf("unsupported source type: %s", sourceType)
	}
	if !compat[credType] {
		return fmt.Errorf("credential type %s is not compatible with source type %s", credType, sourceType)
	}
	if sourceType == SourceTypeGit {
		ssh := isSSHURL(url)
		if credType == CredentialTypeSSHKey && !ssh {
			return fmt.Errorf("ssh_key credential requires an SSH git URL (ssh:// or git@host:); got %s", url)
		}
		if credType == CredentialTypeBasicAuth && ssh {
			return fmt.Errorf("basic_auth credential requires an HTTPS git URL; got SSH URL %s", url)
		}
	}
	return nil
}

// isSSHURL detects SSH-style git URLs: ssh:// prefix or SCP-style (git@host:path).
func isSSHURL(target string) bool {
	if strings.HasPrefix(target, "ssh://") {
		return true
	}
	if strings.Contains(target, "://") {
		return false
	}
	at := strings.Index(target, "@")
	colon := strings.Index(target, ":")
	return at >= 0 && colon > at
}

type Source struct {
	Id             uuid.UUID    `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Name           string       `gorm:"uniqueIndex;not null"`
	Description    string       `gorm:"type:text"`
	Type           string       `gorm:"not null"`
	URL            string       `gorm:"column:url;not null"`
	CredentialId   *uuid.UUID   `gorm:"type:uuid"`
	Catalog        bool         `gorm:"not null;default:false"`
	SourceConfig   SourceConfig `gorm:"type:jsonb;not null;default:'{}'"`
	Labels         Labels       `gorm:"type:jsonb;default:'{}'"`
	LastTestStatus *string      `gorm:"type:text"`
	LastTestError  string       `gorm:"type:text;not null;default:''"`
	LastTestedAt   *time.Time
	CreatedBy      string `gorm:"not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      gorm.DeletedAt `gorm:"index"`
	CreatedByName  string         `gorm:"->;column:created_by_name"`
	CreatedByEmail string         `gorm:"->;column:created_by_email"`
}

func (s *Source) Validate() error {
	if err := ValidateSlug(s.Name); err != nil {
		return fmt.Errorf("invalid name: %w", err)
	}
	switch s.Type {
	case SourceTypeGit, SourceTypeTerraform, SourceTypeHelm, SourceTypeOCI, SourceTypeHTTP:
	case "":
		return fmt.Errorf("type is required")
	default:
		return fmt.Errorf("invalid type: %s", s.Type)
	}
	if strings.TrimSpace(s.URL) == "" {
		return fmt.Errorf("url is required")
	}
	if err := s.SourceConfig.Validate(s.Type); err != nil {
		return err
	}
	if err := s.Labels.Validate(); err != nil {
		return err
	}
	if s.CreatedBy == "" {
		return fmt.Errorf("created_by is required")
	}
	return nil
}

func (s *Source) ToProto() *sourcev1.Source {
	out := &sourcev1.Source{
		Id:            s.Id.String(),
		Name:          s.Name,
		Description:   s.Description,
		Type:          sourceTypeToProto[s.Type],
		Url:           s.URL,
		Catalog:       s.Catalog,
		Labels:        map[string]string(s.Labels),
		LastTestError: s.LastTestError,
		CreatedBy:     &commonv1.ActorRef{Id: s.CreatedBy, DisplayName: s.CreatedByName, Email: s.CreatedByEmail},
		CreatedAt:     timestamppb.New(s.CreatedAt),
		UpdatedAt:     timestamppb.New(s.UpdatedAt),
	}

	if s.CredentialId != nil {
		id := s.CredentialId.String()
		out.CredentialId = &id
	}
	if s.LastTestStatus != nil {
		status := sourceTestStatusToProto[*s.LastTestStatus]
		out.LastTestStatus = &status
	}
	if s.LastTestedAt != nil {
		out.LastTestedAt = timestamppb.New(*s.LastTestedAt)
	}

	out.SourceConfig = s.SourceConfig.ToProto()

	return out
}
