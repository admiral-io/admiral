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
	sourcev1 "go.admiral.io/sdk/proto/admiral/source/v1"
)

const (
	SourceConfigKindTerraform = "terraform"
	SourceConfigKindHelm      = "helm"
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
		CreatedBy:     &commonv1.ActorRef{Id: s.CreatedBy},
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
