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

// Auth config types stored in the JSONB "type" discriminator.
const (
	AuthTypeGitToken        = "git_token"
	AuthTypeGitSSH          = "git_ssh"
	AuthTypeGitGitHubApp    = "git_github_app"
	AuthTypeHelmHTTP        = "helm_http"
	AuthTypeOCIBasic        = "oci_basic"
	AuthTypeAWSFederation   = "aws_federation"
	AuthTypeGCPFederation   = "gcp_federation"
	AuthTypeAzureFederation = "azure_federation"
	AuthTypeAWSStatic       = "aws_static"
	AuthTypeGCPStatic       = "gcp_static"
	AuthTypeAzureStatic     = "azure_static"
	AuthTypeBearerToken     = "bearer_token"
)

// CredentialType string constants matching the CHECK constraint values.
const (
	CredentialTypeGitToken          = "GIT_TOKEN"
	CredentialTypeGitSSH            = "GIT_SSH"
	CredentialTypeGitGitHubApp      = "GIT_GITHUB_APP"
	CredentialTypeHelmHTTP          = "HELM_HTTP"
	CredentialTypeOCIBasic          = "OCI_BASIC"
	CredentialTypeCloudAWS          = "CLOUD_AWS"
	CredentialTypeCloudGCP          = "CLOUD_GCP"
	CredentialTypeCloudAzure        = "CLOUD_AZURE"
	CredentialTypeCloudAWSStatic    = "CLOUD_AWS_STATIC"
	CredentialTypeCloudGCPStatic    = "CLOUD_GCP_STATIC"
	CredentialTypeCloudAzureStatic  = "CLOUD_AZURE_STATIC"
	CredentialTypeTerraformRegistry = "TERRAFORM_REGISTRY"
	CredentialTypeS3                = "S3"
	CredentialTypeGCS               = "GCS"
)

// GitTokenAuth holds HTTPS token credentials for Git operations.
type GitTokenAuth struct {
	Token string `json:"token"`
}

// GitSSHAuth holds SSH key credentials for Git operations.
type GitSSHAuth struct {
	PrivateKey string `json:"private_key"`
	Passphrase string `json:"passphrase,omitempty"`
}

// GitHubAppAuth holds GitHub App credentials for Git operations.
type GitHubAppAuth struct {
	AppId          string `json:"app_id"`
	InstallationId string `json:"installation_id"`
	PrivateKey     string `json:"private_key"`
}

// HelmHTTPAuth holds basic auth credentials for Helm HTTP repositories.
type HelmHTTPAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// OCIBasicAuth holds basic auth credentials for OCI registries.
type OCIBasicAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// AWSFederationConfig holds IAM role configuration for OIDC federation.
type AWSFederationConfig struct {
	RoleArn    string `json:"role_arn"`
	ExternalId string `json:"external_id,omitempty"`
	Region     string `json:"region,omitempty"`
}

// GCPFederationConfig holds Workload Identity Federation configuration.
type GCPFederationConfig struct {
	ProjectNumber       string `json:"project_number"`
	PoolId              string `json:"pool_id"`
	ProviderId          string `json:"provider_id"`
	ServiceAccountEmail string `json:"service_account_email"`
}

// AzureFederationConfig holds Azure federated credential configuration.
type AzureFederationConfig struct {
	AzureTenantId string `json:"azure_tenant_id"`
	ClientId      string `json:"client_id"`
}

// AWSStaticAuth holds static AWS credentials.
type AWSStaticAuth struct {
	AccessKeyId     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	Region          string `json:"region,omitempty"`
}

// GCPStaticAuth holds a static GCP service account key.
type GCPStaticAuth struct {
	ServiceAccountJson string `json:"service_account_json"`
}

// AzureStaticAuth holds static Azure service principal credentials.
type AzureStaticAuth struct {
	AzureTenantId string `json:"azure_tenant_id"`
	ClientId      string `json:"client_id"`
	ClientSecret  string `json:"client_secret"`
}

// BearerTokenAuth holds a bearer token for token-based authentication.
type BearerTokenAuth struct {
	Token string `json:"token"`
}

// AuthConfig is the JSONB-backed polymorphic auth configuration.
// The Type field is the discriminator; exactly one of the pointer fields is non-nil.
type AuthConfig struct {
	Type            string                 `json:"type"`
	GitToken        *GitTokenAuth          `json:"git_token,omitempty"`
	GitSSH          *GitSSHAuth            `json:"git_ssh,omitempty"`
	GitGitHubApp    *GitHubAppAuth         `json:"git_github_app,omitempty"`
	HelmHTTP        *HelmHTTPAuth          `json:"helm_http,omitempty"`
	OCIBasic        *OCIBasicAuth          `json:"oci_basic,omitempty"`
	AWSFederation   *AWSFederationConfig   `json:"aws_federation,omitempty"`
	GCPFederation   *GCPFederationConfig   `json:"gcp_federation,omitempty"`
	AzureFederation *AzureFederationConfig `json:"azure_federation,omitempty"`
	AWSStatic       *AWSStaticAuth         `json:"aws_static,omitempty"`
	GCPStatic       *GCPStaticAuth         `json:"gcp_static,omitempty"`
	AzureStatic     *AzureStaticAuth       `json:"azure_static,omitempty"`
	BearerToken     *BearerTokenAuth       `json:"bearer_token,omitempty"`
}

func (a AuthConfig) Value() (driver.Value, error) {
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
	return json.Unmarshal(bytes, a)
}

// Credential represents stored authentication configuration for accessing
// an external system. Credentials are tenant-scoped and referenced by sources
// when fetching artifacts.
type Credential struct {
	Id          uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Name        string         `gorm:"uniqueIndex;not null"`
	Description string         `gorm:"type:text"`
	Type        string         `gorm:"not null"`
	AuthConfig  AuthConfig     `gorm:"type:jsonb;not null;default:'{}'"`
	Labels      Labels         `gorm:"type:jsonb;default:'{}'"`
	CreatedBy   string         `gorm:"not null"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}


// credentialTypeToProto maps DB type strings to proto ConnectionType values.
var credentialTypeToProto = map[string]credentialv1.CredentialType{
	CredentialTypeGitToken:          credentialv1.CredentialType_CREDENTIAL_TYPE_GIT_TOKEN,
	CredentialTypeGitSSH:            credentialv1.CredentialType_CREDENTIAL_TYPE_GIT_SSH,
	CredentialTypeGitGitHubApp:      credentialv1.CredentialType_CREDENTIAL_TYPE_GIT_GITHUB_APP,
	CredentialTypeHelmHTTP:          credentialv1.CredentialType_CREDENTIAL_TYPE_HELM_HTTP,
	CredentialTypeOCIBasic:          credentialv1.CredentialType_CREDENTIAL_TYPE_OCI_BASIC,
	CredentialTypeCloudAWS:          credentialv1.CredentialType_CREDENTIAL_TYPE_CLOUD_AWS,
	CredentialTypeCloudGCP:          credentialv1.CredentialType_CREDENTIAL_TYPE_CLOUD_GCP,
	CredentialTypeCloudAzure:        credentialv1.CredentialType_CREDENTIAL_TYPE_CLOUD_AZURE,
	CredentialTypeCloudAWSStatic:    credentialv1.CredentialType_CREDENTIAL_TYPE_CLOUD_AWS_STATIC,
	CredentialTypeCloudGCPStatic:    credentialv1.CredentialType_CREDENTIAL_TYPE_CLOUD_GCP_STATIC,
	CredentialTypeCloudAzureStatic:  credentialv1.CredentialType_CREDENTIAL_TYPE_CLOUD_AZURE_STATIC,
	CredentialTypeTerraformRegistry: credentialv1.CredentialType_CREDENTIAL_TYPE_TERRAFORM_REGISTRY,
	CredentialTypeS3:                credentialv1.CredentialType_CREDENTIAL_TYPE_S3,
	CredentialTypeGCS:               credentialv1.CredentialType_CREDENTIAL_TYPE_GCS,
}

// CredentialTypeFromProto maps a proto ConnectionType to the DB string.
func CredentialTypeFromProto(t credentialv1.CredentialType) string {
	for k, v := range credentialTypeToProto {
		if v == t {
			return k
		}
	}
	return ""
}

func (c *Credential) ToProto() *credentialv1.Credential {
	conn := &credentialv1.Credential{
		Id:          c.Id.String(),
		Name:        c.Name,
		Description: c.Description,
		Type:        credentialTypeToProto[c.Type],
		Labels:      map[string]string(c.Labels),
		CreatedBy:   &commonv1.ActorRef{Id: c.CreatedBy},
		CreatedAt:   timestamppb.New(c.CreatedAt),
		UpdatedAt:   timestamppb.New(c.UpdatedAt),
	}

	c.AuthConfig.setProtoAuthConfig(conn)

	return conn
}

// setProtoAuthConfig sets the auth_config oneof on a proto Connection.
// Sensitive fields (tokens, keys, passwords) are omitted (write-only).
func (a *AuthConfig) setProtoAuthConfig(conn *credentialv1.Credential) {
	if a == nil {
		return
	}
	switch a.Type {
	case AuthTypeGitToken:
		if a.GitToken != nil {
			conn.AuthConfig = &credentialv1.Credential_GitToken{
				GitToken: &credentialv1.GitTokenAuth{},
			}
		}
	case AuthTypeGitSSH:
		if a.GitSSH != nil {
			conn.AuthConfig = &credentialv1.Credential_GitSsh{
				GitSsh: &credentialv1.GitSSHAuth{},
			}
		}
	case AuthTypeGitGitHubApp:
		if a.GitGitHubApp != nil {
			conn.AuthConfig = &credentialv1.Credential_GitGithubApp{
				GitGithubApp: &credentialv1.GitHubAppAuth{
					AppId:          a.GitGitHubApp.AppId,
					InstallationId: a.GitGitHubApp.InstallationId,
				},
			}
		}
	case AuthTypeHelmHTTP:
		if a.HelmHTTP != nil {
			conn.AuthConfig = &credentialv1.Credential_HelmHttp{
				HelmHttp: &credentialv1.HelmHTTPAuth{
					Username: a.HelmHTTP.Username,
				},
			}
		}
	case AuthTypeOCIBasic:
		if a.OCIBasic != nil {
			conn.AuthConfig = &credentialv1.Credential_OciBasic{
				OciBasic: &credentialv1.BasicAuth{
					Username: a.OCIBasic.Username,
				},
			}
		}
	case AuthTypeAWSFederation:
		if a.AWSFederation != nil {
			conn.AuthConfig = &credentialv1.Credential_AwsFederation{
				AwsFederation: &credentialv1.AWSFederationConfig{
					RoleArn:    a.AWSFederation.RoleArn,
					ExternalId: a.AWSFederation.ExternalId,
					Region:     a.AWSFederation.Region,
				},
			}
		}
	case AuthTypeGCPFederation:
		if a.GCPFederation != nil {
			conn.AuthConfig = &credentialv1.Credential_GcpFederation{
				GcpFederation: &credentialv1.GCPFederationConfig{
					ProjectNumber:       a.GCPFederation.ProjectNumber,
					PoolId:              a.GCPFederation.PoolId,
					ProviderId:          a.GCPFederation.ProviderId,
					ServiceAccountEmail: a.GCPFederation.ServiceAccountEmail,
				},
			}
		}
	case AuthTypeAzureFederation:
		if a.AzureFederation != nil {
			conn.AuthConfig = &credentialv1.Credential_AzureFederation{
				AzureFederation: &credentialv1.AzureFederationConfig{
					AzureTenantId: a.AzureFederation.AzureTenantId,
					ClientId:      a.AzureFederation.ClientId,
				},
			}
		}
	case AuthTypeAWSStatic:
		if a.AWSStatic != nil {
			conn.AuthConfig = &credentialv1.Credential_AwsStatic{ //nolint:staticcheck
				AwsStatic: &credentialv1.AWSStaticAuth{ //nolint:staticcheck
					AccessKeyId: a.AWSStatic.AccessKeyId,
					Region:      a.AWSStatic.Region,
				},
			}
		}
	case AuthTypeGCPStatic:
		if a.GCPStatic != nil {
			conn.AuthConfig = &credentialv1.Credential_GcpStatic{ //nolint:staticcheck
				GcpStatic: &credentialv1.GCPStaticAuth{}, //nolint:staticcheck
			}
		}
	case AuthTypeAzureStatic:
		if a.AzureStatic != nil {
			conn.AuthConfig = &credentialv1.Credential_AzureStatic{ //nolint:staticcheck
				AzureStatic: &credentialv1.AzureStaticAuth{ //nolint:staticcheck
					AzureTenantId: a.AzureStatic.AzureTenantId,
					ClientId:      a.AzureStatic.ClientId,
				},
			}
		}
	case AuthTypeBearerToken:
		if a.BearerToken != nil {
			conn.AuthConfig = &credentialv1.Credential_BearerToken{
				BearerToken: &credentialv1.BearerTokenAuth{},
			}
		}
	}
}

// AuthConfigFromProto converts a proto Connection's auth_config oneof to the model.
func AuthConfigFromProto(conn *credentialv1.Credential) AuthConfig {
	switch c := conn.GetAuthConfig().(type) {
	case *credentialv1.Credential_GitToken:
		return AuthConfig{Type: AuthTypeGitToken, GitToken: &GitTokenAuth{Token: c.GitToken.GetToken()}}
	case *credentialv1.Credential_GitSsh:
		return AuthConfig{Type: AuthTypeGitSSH, GitSSH: &GitSSHAuth{PrivateKey: c.GitSsh.GetPrivateKey(), Passphrase: c.GitSsh.GetPassphrase()}}
	case *credentialv1.Credential_GitGithubApp:
		return AuthConfig{Type: AuthTypeGitGitHubApp, GitGitHubApp: &GitHubAppAuth{AppId: c.GitGithubApp.GetAppId(), InstallationId: c.GitGithubApp.GetInstallationId(), PrivateKey: c.GitGithubApp.GetPrivateKey()}}
	case *credentialv1.Credential_HelmHttp:
		return AuthConfig{Type: AuthTypeHelmHTTP, HelmHTTP: &HelmHTTPAuth{Username: c.HelmHttp.GetUsername(), Password: c.HelmHttp.GetPassword()}}
	case *credentialv1.Credential_OciBasic:
		return AuthConfig{Type: AuthTypeOCIBasic, OCIBasic: &OCIBasicAuth{Username: c.OciBasic.GetUsername(), Password: c.OciBasic.GetPassword()}}
	case *credentialv1.Credential_AwsFederation:
		return AuthConfig{Type: AuthTypeAWSFederation, AWSFederation: &AWSFederationConfig{RoleArn: c.AwsFederation.GetRoleArn(), ExternalId: c.AwsFederation.GetExternalId(), Region: c.AwsFederation.GetRegion()}}
	case *credentialv1.Credential_GcpFederation:
		return AuthConfig{Type: AuthTypeGCPFederation, GCPFederation: &GCPFederationConfig{ProjectNumber: c.GcpFederation.GetProjectNumber(), PoolId: c.GcpFederation.GetPoolId(), ProviderId: c.GcpFederation.GetProviderId(), ServiceAccountEmail: c.GcpFederation.GetServiceAccountEmail()}}
	case *credentialv1.Credential_AzureFederation:
		return AuthConfig{Type: AuthTypeAzureFederation, AzureFederation: &AzureFederationConfig{AzureTenantId: c.AzureFederation.GetAzureTenantId(), ClientId: c.AzureFederation.GetClientId()}}
	case *credentialv1.Credential_AwsStatic: //nolint:staticcheck
		return AuthConfig{Type: AuthTypeAWSStatic, AWSStatic: &AWSStaticAuth{AccessKeyId: c.AwsStatic.GetAccessKeyId(), SecretAccessKey: c.AwsStatic.GetSecretAccessKey(), Region: c.AwsStatic.GetRegion()}}
	case *credentialv1.Credential_GcpStatic: //nolint:staticcheck
		return AuthConfig{Type: AuthTypeGCPStatic, GCPStatic: &GCPStaticAuth{ServiceAccountJson: c.GcpStatic.GetServiceAccountJson()}}
	case *credentialv1.Credential_AzureStatic: //nolint:staticcheck
		return AuthConfig{Type: AuthTypeAzureStatic, AzureStatic: &AzureStaticAuth{AzureTenantId: c.AzureStatic.GetAzureTenantId(), ClientId: c.AzureStatic.GetClientId(), ClientSecret: c.AzureStatic.GetClientSecret()}}
	case *credentialv1.Credential_BearerToken:
		return AuthConfig{Type: AuthTypeBearerToken, BearerToken: &BearerTokenAuth{Token: c.BearerToken.GetToken()}}
	}
	return AuthConfig{}
}

// AuthConfigFromCreateRequest converts a CreateCredentialRequest's auth_config oneof to the model.
func AuthConfigFromCreateRequest(req *credentialv1.CreateCredentialRequest) AuthConfig {
	switch c := req.GetAuthConfig().(type) {
	case *credentialv1.CreateCredentialRequest_GitToken:
		return AuthConfig{Type: AuthTypeGitToken, GitToken: &GitTokenAuth{Token: c.GitToken.GetToken()}}
	case *credentialv1.CreateCredentialRequest_GitSsh:
		return AuthConfig{Type: AuthTypeGitSSH, GitSSH: &GitSSHAuth{PrivateKey: c.GitSsh.GetPrivateKey(), Passphrase: c.GitSsh.GetPassphrase()}}
	case *credentialv1.CreateCredentialRequest_GitGithubApp:
		return AuthConfig{Type: AuthTypeGitGitHubApp, GitGitHubApp: &GitHubAppAuth{AppId: c.GitGithubApp.GetAppId(), InstallationId: c.GitGithubApp.GetInstallationId(), PrivateKey: c.GitGithubApp.GetPrivateKey()}}
	case *credentialv1.CreateCredentialRequest_HelmHttp:
		return AuthConfig{Type: AuthTypeHelmHTTP, HelmHTTP: &HelmHTTPAuth{Username: c.HelmHttp.GetUsername(), Password: c.HelmHttp.GetPassword()}}
	case *credentialv1.CreateCredentialRequest_OciBasic:
		return AuthConfig{Type: AuthTypeOCIBasic, OCIBasic: &OCIBasicAuth{Username: c.OciBasic.GetUsername(), Password: c.OciBasic.GetPassword()}}
	case *credentialv1.CreateCredentialRequest_AwsFederation:
		return AuthConfig{Type: AuthTypeAWSFederation, AWSFederation: &AWSFederationConfig{RoleArn: c.AwsFederation.GetRoleArn(), ExternalId: c.AwsFederation.GetExternalId(), Region: c.AwsFederation.GetRegion()}}
	case *credentialv1.CreateCredentialRequest_GcpFederation:
		return AuthConfig{Type: AuthTypeGCPFederation, GCPFederation: &GCPFederationConfig{ProjectNumber: c.GcpFederation.GetProjectNumber(), PoolId: c.GcpFederation.GetPoolId(), ProviderId: c.GcpFederation.GetProviderId(), ServiceAccountEmail: c.GcpFederation.GetServiceAccountEmail()}}
	case *credentialv1.CreateCredentialRequest_AzureFederation:
		return AuthConfig{Type: AuthTypeAzureFederation, AzureFederation: &AzureFederationConfig{AzureTenantId: c.AzureFederation.GetAzureTenantId(), ClientId: c.AzureFederation.GetClientId()}}
	case *credentialv1.CreateCredentialRequest_AwsStatic: //nolint:staticcheck
		return AuthConfig{Type: AuthTypeAWSStatic, AWSStatic: &AWSStaticAuth{AccessKeyId: c.AwsStatic.GetAccessKeyId(), SecretAccessKey: c.AwsStatic.GetSecretAccessKey(), Region: c.AwsStatic.GetRegion()}}
	case *credentialv1.CreateCredentialRequest_GcpStatic: //nolint:staticcheck
		return AuthConfig{Type: AuthTypeGCPStatic, GCPStatic: &GCPStaticAuth{ServiceAccountJson: c.GcpStatic.GetServiceAccountJson()}}
	case *credentialv1.CreateCredentialRequest_AzureStatic: //nolint:staticcheck
		return AuthConfig{Type: AuthTypeAzureStatic, AzureStatic: &AzureStaticAuth{AzureTenantId: c.AzureStatic.GetAzureTenantId(), ClientId: c.AzureStatic.GetClientId(), ClientSecret: c.AzureStatic.GetClientSecret()}}
	case *credentialv1.CreateCredentialRequest_BearerToken:
		return AuthConfig{Type: AuthTypeBearerToken, BearerToken: &BearerTokenAuth{Token: c.BearerToken.GetToken()}}
	}
	return AuthConfig{}
}
