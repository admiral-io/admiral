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

// Source config discriminators stored in the JSONB "type" field.
const (
	SourceConfigTypeTerraformRegistry = "terraform_registry"
	SourceConfigTypeTerraformGit      = "terraform_git"
	SourceConfigTypeHelmRepository    = "helm_repository"
	SourceConfigTypeHelmOCI           = "helm_oci"
	SourceConfigTypeHelmGit           = "helm_git"
	SourceConfigTypeKustomizeGit      = "kustomize_git"
	SourceConfigTypeManifestGit       = "manifest_git"
	SourceConfigTypeArchive           = "archive"
)

// SourceType string constants matching the CHECK constraint values.
const (
	SourceTypeTerraformRegistry = "TERRAFORM_REGISTRY"
	SourceTypeTerraformGit      = "TERRAFORM_GIT"
	SourceTypeHelmRepository    = "HELM_REPOSITORY"
	SourceTypeHelmOCI           = "HELM_OCI"
	SourceTypeHelmGit           = "HELM_GIT"
	SourceTypeKustomizeGit      = "KUSTOMIZE_GIT"
	SourceTypeManifestGit       = "MANIFEST_GIT"
	SourceTypeArchive           = "ARCHIVE"
)

// Source test status values matching the CHECK constraint. Null in the DB
// represents "never tested" (proto SOURCE_TEST_STATUS_UNSPECIFIED).
const (
	SourceTestStatusSuccess = "SUCCESS"
	SourceTestStatusFailure = "FAILURE"
)

type TerraformRegistryConfig struct {
	Namespace  string `json:"namespace"`
	ModuleName string `json:"module_name"`
	System     string `json:"system"`
}

type TerraformGitConfig struct {
	Path       string `json:"path,omitempty"`
	DefaultRef string `json:"default_ref,omitempty"`
}

type HelmRepositoryConfig struct {
	ChartName string `json:"chart_name"`
}

type HelmOCIConfig struct {
	Repository string `json:"repository"`
}

type HelmGitConfig struct {
	Path       string `json:"path"`
	DefaultRef string `json:"default_ref,omitempty"`
}

type KustomizeGitConfig struct {
	Path       string `json:"path"`
	DefaultRef string `json:"default_ref,omitempty"`
}

type ManifestGitConfig struct {
	Path       string `json:"path"`
	Recursive  bool   `json:"recursive,omitempty"`
	DefaultRef string `json:"default_ref,omitempty"`
}

type ArchiveConfig struct {
	Path     string `json:"path,omitempty"`
	Checksum string `json:"checksum,omitempty"`
}

// SourceConfig is the JSONB-backed polymorphic per-type configuration.
// The Type field is the discriminator; exactly one of the pointer fields is
// non-nil when populated.
type SourceConfig struct {
	Type               string                   `json:"type,omitempty"`
	TerraformRegistry  *TerraformRegistryConfig `json:"terraform_registry,omitempty"`
	TerraformGit       *TerraformGitConfig      `json:"terraform_git,omitempty"`
	HelmRepository     *HelmRepositoryConfig    `json:"helm_repository,omitempty"`
	HelmOCI            *HelmOCIConfig           `json:"helm_oci,omitempty"`
	HelmGit            *HelmGitConfig           `json:"helm_git,omitempty"`
	KustomizeGit       *KustomizeGitConfig      `json:"kustomize_git,omitempty"`
	ManifestGit        *ManifestGitConfig       `json:"manifest_git,omitempty"`
	Archive            *ArchiveConfig           `json:"archive,omitempty"`
}

func (s SourceConfig) Value() (driver.Value, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal source config: %w", err)
	}
	return string(b), nil
}

func (s *SourceConfig) Scan(value any) error {
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
	return json.Unmarshal(bytes, s)
}

// Source represents a registered pointer to an external artifact location.
// Sources carry the URL and (optionally) reference a Credential for auth.
type Source struct {
	Id             uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Name           string         `gorm:"uniqueIndex;not null"`
	Description    string         `gorm:"type:text"`
	Type           string         `gorm:"not null"`
	URL            string         `gorm:"column:url;not null"`
	CredentialId   *uuid.UUID     `gorm:"type:uuid"`
	Catalog        bool           `gorm:"not null;default:false"`
	SourceConfig   SourceConfig   `gorm:"type:jsonb;not null;default:'{}'"`
	Labels         Labels         `gorm:"type:jsonb;default:'{}'"`
	LastTestStatus *string        `gorm:"type:text"`
	LastTestError  string         `gorm:"type:text;not null;default:''"`
	LastTestedAt   *time.Time
	LastSyncedAt   *time.Time
	CreatedBy      string         `gorm:"not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      gorm.DeletedAt `gorm:"index"`
}

var sourceTypeToProto = map[string]sourcev1.SourceType{
	SourceTypeTerraformRegistry: sourcev1.SourceType_SOURCE_TYPE_TERRAFORM_REGISTRY,
	SourceTypeTerraformGit:      sourcev1.SourceType_SOURCE_TYPE_TERRAFORM_GIT,
	SourceTypeHelmRepository:    sourcev1.SourceType_SOURCE_TYPE_HELM_REPOSITORY,
	SourceTypeHelmOCI:           sourcev1.SourceType_SOURCE_TYPE_HELM_OCI,
	SourceTypeHelmGit:           sourcev1.SourceType_SOURCE_TYPE_HELM_GIT,
	SourceTypeKustomizeGit:      sourcev1.SourceType_SOURCE_TYPE_KUSTOMIZE_GIT,
	SourceTypeManifestGit:       sourcev1.SourceType_SOURCE_TYPE_MANIFEST_GIT,
	SourceTypeArchive:           sourcev1.SourceType_SOURCE_TYPE_ARCHIVE,
}

func SourceTypeFromProto(t sourcev1.SourceType) string {
	for k, v := range sourceTypeToProto {
		if v == t {
			return k
		}
	}
	return ""
}

var sourceTestStatusToProto = map[string]sourcev1.SourceTestStatus{
	SourceTestStatusSuccess: sourcev1.SourceTestStatus_SOURCE_TEST_STATUS_SUCCESS,
	SourceTestStatusFailure: sourcev1.SourceTestStatus_SOURCE_TEST_STATUS_FAILURE,
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
	if s.LastSyncedAt != nil {
		out.LastSyncedAt = timestamppb.New(*s.LastSyncedAt)
	}

	s.SourceConfig.setProtoSourceConfig(out)

	return out
}

func (c *SourceConfig) setProtoSourceConfig(out *sourcev1.Source) {
	if c == nil {
		return
	}
	switch c.Type {
	case SourceConfigTypeTerraformRegistry:
		if c.TerraformRegistry != nil {
			out.SourceConfig = &sourcev1.Source_TerraformRegistry{
				TerraformRegistry: &sourcev1.TerraformRegistryConfig{
					Namespace:  c.TerraformRegistry.Namespace,
					ModuleName: c.TerraformRegistry.ModuleName,
					System:     c.TerraformRegistry.System,
				},
			}
		}
	case SourceConfigTypeTerraformGit:
		if c.TerraformGit != nil {
			out.SourceConfig = &sourcev1.Source_TerraformGit{
				TerraformGit: &sourcev1.TerraformGitConfig{
					Path:       c.TerraformGit.Path,
					DefaultRef: c.TerraformGit.DefaultRef,
				},
			}
		}
	case SourceConfigTypeHelmRepository:
		if c.HelmRepository != nil {
			out.SourceConfig = &sourcev1.Source_HelmRepository{
				HelmRepository: &sourcev1.HelmRepositoryConfig{
					ChartName: c.HelmRepository.ChartName,
				},
			}
		}
	case SourceConfigTypeHelmOCI:
		if c.HelmOCI != nil {
			out.SourceConfig = &sourcev1.Source_HelmOci{
				HelmOci: &sourcev1.HelmOCIConfig{
					Repository: c.HelmOCI.Repository,
				},
			}
		}
	case SourceConfigTypeHelmGit:
		if c.HelmGit != nil {
			out.SourceConfig = &sourcev1.Source_HelmGit{
				HelmGit: &sourcev1.HelmGitConfig{
					Path:       c.HelmGit.Path,
					DefaultRef: c.HelmGit.DefaultRef,
				},
			}
		}
	case SourceConfigTypeKustomizeGit:
		if c.KustomizeGit != nil {
			out.SourceConfig = &sourcev1.Source_KustomizeGit{
				KustomizeGit: &sourcev1.KustomizeGitConfig{
					Path:       c.KustomizeGit.Path,
					DefaultRef: c.KustomizeGit.DefaultRef,
				},
			}
		}
	case SourceConfigTypeManifestGit:
		if c.ManifestGit != nil {
			out.SourceConfig = &sourcev1.Source_ManifestGit{
				ManifestGit: &sourcev1.ManifestGitConfig{
					Path:       c.ManifestGit.Path,
					Recursive:  c.ManifestGit.Recursive,
					DefaultRef: c.ManifestGit.DefaultRef,
				},
			}
		}
	case SourceConfigTypeArchive:
		if c.Archive != nil {
			out.SourceConfig = &sourcev1.Source_Archive{
				Archive: &sourcev1.ArchiveConfig{
					Path:     c.Archive.Path,
					Checksum: c.Archive.Checksum,
				},
			}
		}
	}
}

func SourceConfigFromProto(src *sourcev1.Source) SourceConfig {
	switch c := src.GetSourceConfig().(type) {
	case *sourcev1.Source_TerraformRegistry:
		return SourceConfig{Type: SourceConfigTypeTerraformRegistry, TerraformRegistry: &TerraformRegistryConfig{
			Namespace: c.TerraformRegistry.GetNamespace(), ModuleName: c.TerraformRegistry.GetModuleName(), System: c.TerraformRegistry.GetSystem(),
		}}
	case *sourcev1.Source_TerraformGit:
		return SourceConfig{Type: SourceConfigTypeTerraformGit, TerraformGit: &TerraformGitConfig{
			Path: c.TerraformGit.GetPath(), DefaultRef: c.TerraformGit.GetDefaultRef(),
		}}
	case *sourcev1.Source_HelmRepository:
		return SourceConfig{Type: SourceConfigTypeHelmRepository, HelmRepository: &HelmRepositoryConfig{
			ChartName: c.HelmRepository.GetChartName(),
		}}
	case *sourcev1.Source_HelmOci:
		return SourceConfig{Type: SourceConfigTypeHelmOCI, HelmOCI: &HelmOCIConfig{
			Repository: c.HelmOci.GetRepository(),
		}}
	case *sourcev1.Source_HelmGit:
		return SourceConfig{Type: SourceConfigTypeHelmGit, HelmGit: &HelmGitConfig{
			Path: c.HelmGit.GetPath(), DefaultRef: c.HelmGit.GetDefaultRef(),
		}}
	case *sourcev1.Source_KustomizeGit:
		return SourceConfig{Type: SourceConfigTypeKustomizeGit, KustomizeGit: &KustomizeGitConfig{
			Path: c.KustomizeGit.GetPath(), DefaultRef: c.KustomizeGit.GetDefaultRef(),
		}}
	case *sourcev1.Source_ManifestGit:
		return SourceConfig{Type: SourceConfigTypeManifestGit, ManifestGit: &ManifestGitConfig{
			Path: c.ManifestGit.GetPath(), Recursive: c.ManifestGit.GetRecursive(), DefaultRef: c.ManifestGit.GetDefaultRef(),
		}}
	case *sourcev1.Source_Archive:
		return SourceConfig{Type: SourceConfigTypeArchive, Archive: &ArchiveConfig{
			Path: c.Archive.GetPath(), Checksum: c.Archive.GetChecksum(),
		}}
	}
	return SourceConfig{}
}

func SourceConfigFromCreateRequest(req *sourcev1.CreateSourceRequest) SourceConfig {
	switch c := req.GetSourceConfig().(type) {
	case *sourcev1.CreateSourceRequest_TerraformRegistry:
		return SourceConfig{Type: SourceConfigTypeTerraformRegistry, TerraformRegistry: &TerraformRegistryConfig{
			Namespace: c.TerraformRegistry.GetNamespace(), ModuleName: c.TerraformRegistry.GetModuleName(), System: c.TerraformRegistry.GetSystem(),
		}}
	case *sourcev1.CreateSourceRequest_TerraformGit:
		return SourceConfig{Type: SourceConfigTypeTerraformGit, TerraformGit: &TerraformGitConfig{
			Path: c.TerraformGit.GetPath(), DefaultRef: c.TerraformGit.GetDefaultRef(),
		}}
	case *sourcev1.CreateSourceRequest_HelmRepository:
		return SourceConfig{Type: SourceConfigTypeHelmRepository, HelmRepository: &HelmRepositoryConfig{
			ChartName: c.HelmRepository.GetChartName(),
		}}
	case *sourcev1.CreateSourceRequest_HelmOci:
		return SourceConfig{Type: SourceConfigTypeHelmOCI, HelmOCI: &HelmOCIConfig{
			Repository: c.HelmOci.GetRepository(),
		}}
	case *sourcev1.CreateSourceRequest_HelmGit:
		return SourceConfig{Type: SourceConfigTypeHelmGit, HelmGit: &HelmGitConfig{
			Path: c.HelmGit.GetPath(), DefaultRef: c.HelmGit.GetDefaultRef(),
		}}
	case *sourcev1.CreateSourceRequest_KustomizeGit:
		return SourceConfig{Type: SourceConfigTypeKustomizeGit, KustomizeGit: &KustomizeGitConfig{
			Path: c.KustomizeGit.GetPath(), DefaultRef: c.KustomizeGit.GetDefaultRef(),
		}}
	case *sourcev1.CreateSourceRequest_ManifestGit:
		return SourceConfig{Type: SourceConfigTypeManifestGit, ManifestGit: &ManifestGitConfig{
			Path: c.ManifestGit.GetPath(), Recursive: c.ManifestGit.GetRecursive(), DefaultRef: c.ManifestGit.GetDefaultRef(),
		}}
	case *sourcev1.CreateSourceRequest_Archive:
		return SourceConfig{Type: SourceConfigTypeArchive, Archive: &ArchiveConfig{
			Path: c.Archive.GetPath(), Checksum: c.Archive.GetChecksum(),
		}}
	}
	return SourceConfig{}
}
