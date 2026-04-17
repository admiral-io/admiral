package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"go.admiral.io/admiral/internal/model"
)

func init() {
	Register(model.SourceTypeTerraform, func() Backend { return &terraformBackend{} })
}

// terraformBackend handles SOURCE_TYPE_TERRAFORM using the Terraform Module
// Registry Protocol (v1).
//
// Probe: GET {url}/.well-known/terraform.json (service discovery).
// ListVersions: GET /v1/modules/{namespace}/{name}/{system}/versions.
// Fetch: GET /v1/modules/{namespace}/{name}/{system}/{version}/download
//
//	which returns 204 with X-Terraform-Get header pointing at the
//	actual archive URL (typically an HTTPS tarball or a go-getter
//	URL). For v1 we only support direct https:// tarball URLs.
//
// Accepts BEARER_TOKEN only.
type terraformBackend struct{}

// tfVersionsResponse is the subset of /versions response we need.
type tfVersionsResponse struct {
	Modules []struct {
		Versions []struct {
			Version string `json:"version"`
		} `json:"versions"`
	} `json:"modules"`
}

func (b *terraformBackend) Probe(ctx context.Context, cred *model.Credential, src *model.Source) error {
	target, err := tfTarget(src)
	if err != nil {
		return err
	}
	if err := ValidateTargetURL(target); err != nil {
		return err
	}
	probeURL := strings.TrimSuffix(target, "/") + "/.well-known/terraform.json"
	return httpProbeWithCred(ctx, cred, probeURL, supportBearerOnly)
}

func (b *terraformBackend) ListVersions(ctx context.Context, cred *model.Credential, src *model.Source) ([]Version, error) {
	target, err := tfTarget(src)
	if err != nil {
		return nil, err
	}
	cfg, err := tfConfig(src)
	if err != nil {
		return nil, err
	}
	if err := ValidateTargetURL(target); err != nil {
		return nil, err
	}

	versionsURL := fmt.Sprintf("%s/v1/modules/%s/%s/%s/versions",
		strings.TrimSuffix(target, "/"), cfg.Namespace, cfg.ModuleName, cfg.System)

	resp, err := httpGetAuthed(ctx, cred, versionsURL, supportBearerOnly)
	if err != nil {
		return nil, fmt.Errorf("fetch versions: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: status %d", versionsURL, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read versions body: %w", err)
	}

	var tvr tfVersionsResponse
	if err := json.Unmarshal(body, &tvr); err != nil {
		return nil, fmt.Errorf("parse versions response: %w", err)
	}

	var versions []Version
	for _, m := range tvr.Modules {
		for _, v := range m.Versions {
			versions = append(versions, Version{Name: v.Version})
		}
	}
	return versions, nil
}

func (b *terraformBackend) Fetch(ctx context.Context, cred *model.Credential, src *model.Source, opts FetchOptions) (*FetchResult, error) {
	target, err := tfTarget(src)
	if err != nil {
		return nil, err
	}
	cfg, err := tfConfig(src)
	if err != nil {
		return nil, err
	}
	if opts.Ref == "" {
		return nil, fmt.Errorf("terraform backend: version is required (pass via opts.Ref)")
	}
	if err := ValidateTargetURL(target); err != nil {
		return nil, err
	}

	// Step 1: ask the registry where to download.
	downloadEndpoint := fmt.Sprintf("%s/v1/modules/%s/%s/%s/%s/download",
		strings.TrimSuffix(target, "/"), cfg.Namespace, cfg.ModuleName, cfg.System, opts.Ref)

	resp, err := httpGetAuthed(ctx, cred, downloadEndpoint, supportBearerOnly)
	if err != nil {
		return nil, fmt.Errorf("fetch download URL: %w", err)
	}
	// Per spec the registry returns 204 with X-Terraform-Get, but some
	// implementations return 200; accept either.
	archiveURL := resp.Header.Get("X-Terraform-Get")
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: status %d", downloadEndpoint, resp.StatusCode)
	}
	if archiveURL == "" {
		return nil, fmt.Errorf("terraform backend: registry did not return X-Terraform-Get header")
	}

	// Resolve relative URLs against the download endpoint.
	resolvedURL, err := resolveRelativeURL(downloadEndpoint, archiveURL)
	if err != nil {
		return nil, err
	}

	// For v1 we only support direct HTTPS tarballs. go-getter URLs like
	// git::https://... or s3::... are rejected with a clear message.
	if strings.Contains(resolvedURL, "::") {
		return nil, fmt.Errorf("terraform backend: registry returned go-getter URL %q; only direct https tarballs are supported in v1", resolvedURL)
	}

	// Step 2: download the archive. Strip any `?archive=...` query-only
	// indicator some registries use.
	archivePath, removeArchive, err := downloadToFile(ctx, cred, resolvedURL, supportBearerOnly)
	if err != nil {
		return nil, err
	}
	defer removeArchive()

	format := detectArchiveFormat(resolvedURL)
	if format == "" {
		// Default to tar.gz when we can't detect -- the Terraform registry
		// convention is tarballs.
		format = "tar.gz"
	}

	dir, err := os.MkdirTemp("", "admiral-backend-terraform-*")
	if err != nil {
		return nil, fmt.Errorf("terraform backend: create tempdir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	if err := extractArchive(archivePath, format, dir); err != nil {
		cleanup()
		return nil, fmt.Errorf("terraform backend: extract module: %w", err)
	}

	return &FetchResult{
		Dir:      dir,
		Revision: opts.Ref,
		Digest:   "", // registry doesn't standardize a module digest
		Cleanup:  cleanup,
	}, nil
}

func tfTarget(src *model.Source) (string, error) {
	if src == nil || src.URL == "" {
		return "", fmt.Errorf("source URL is required")
	}
	return src.URL, nil
}

func tfConfig(src *model.Source) (*model.TerraformConfig, error) {
	if src == nil || src.SourceConfig.Terraform == nil {
		return nil, fmt.Errorf("terraform backend: source is missing source_config.terraform")
	}
	cfg := src.SourceConfig.Terraform
	if cfg.Namespace == "" || cfg.ModuleName == "" || cfg.System == "" {
		return nil, fmt.Errorf("terraform backend: source_config.terraform requires namespace, module_name, and system")
	}
	return cfg, nil
}
