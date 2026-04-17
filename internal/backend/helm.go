package backend

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"go.admiral.io/admiral/internal/model"
)

func init() {
	Register(model.SourceTypeHelm, func() Backend { return &helmBackend{} })
}

// helmBackend handles SOURCE_TYPE_HELM: the classic HTTP chart repository
// protocol (index.yaml catalog + .tgz chart archives).
//
// Probe: GET {url}/index.yaml.
// ListVersions: parse index.yaml, return entries for source_config.helm.chart_name.
// Fetch: resolve the chart entry matching opts.Ref (empty Ref = first entry,
// which Helm index convention orders newest-first), download the .tgz,
// extract into a tempdir.
//
// Accepts BASIC_AUTH, BEARER_TOKEN, or anonymous.
type helmBackend struct{}

// helmIndex is the minimal shape of a Helm index.yaml we need.
type helmIndex struct {
	APIVersion string                        `yaml:"apiVersion"`
	Entries    map[string][]helmChartVersion `yaml:"entries"`
}

type helmChartVersion struct {
	Version     string    `yaml:"version"`
	AppVersion  string    `yaml:"appVersion"`
	Description string    `yaml:"description"`
	URLs        []string  `yaml:"urls"`
	Created     time.Time `yaml:"created"`
	Digest      string    `yaml:"digest"`
}

func (b *helmBackend) Probe(ctx context.Context, cred *model.Credential, src *model.Source) error {
	target, err := helmTarget(src)
	if err != nil {
		return err
	}
	if err := ValidateTargetURL(target); err != nil {
		return err
	}
	probeURL := strings.TrimSuffix(target, "/") + "/index.yaml"
	return httpProbeWithCred(ctx, cred, probeURL, supportBasicAndBearer)
}

func (b *helmBackend) ListVersions(ctx context.Context, cred *model.Credential, src *model.Source) ([]Version, error) {
	idx, chartName, err := b.loadIndex(ctx, cred, src)
	if err != nil {
		return nil, err
	}
	entries, ok := idx.Entries[chartName]
	if !ok {
		return nil, fmt.Errorf("helm backend: chart %q not found in index", chartName)
	}

	versions := make([]Version, 0, len(entries))
	for _, e := range entries {
		v := Version{Name: e.Version, Description: e.Description}
		if !e.Created.IsZero() {
			t := e.Created
			v.PublishedAt = &t
		}
		versions = append(versions, v)
	}
	return versions, nil
}

func (b *helmBackend) Fetch(ctx context.Context, cred *model.Credential, src *model.Source, opts FetchOptions) (*FetchResult, error) {
	idx, chartName, err := b.loadIndex(ctx, cred, src)
	if err != nil {
		return nil, err
	}
	entries, ok := idx.Entries[chartName]
	if !ok || len(entries) == 0 {
		return nil, fmt.Errorf("helm backend: chart %q not found in index", chartName)
	}

	// Empty ref → first entry (Helm index convention is newest-first).
	var chosen *helmChartVersion
	if opts.Ref == "" {
		chosen = &entries[0]
	} else {
		for i := range entries {
			if entries[i].Version == opts.Ref {
				chosen = &entries[i]
				break
			}
		}
		if chosen == nil {
			return nil, fmt.Errorf("helm backend: chart %q has no version %q", chartName, opts.Ref)
		}
	}
	if len(chosen.URLs) == 0 {
		return nil, fmt.Errorf("helm backend: chart %q version %q has no download URL", chartName, chosen.Version)
	}

	tgzURL, err := resolveRelativeURL(strings.TrimSuffix(src.URL, "/")+"/index.yaml", chosen.URLs[0])
	if err != nil {
		return nil, err
	}

	archivePath, removeArchive, err := downloadToFile(ctx, cred, tgzURL, supportBasicAndBearer)
	if err != nil {
		return nil, err
	}
	defer removeArchive()

	dir, err := os.MkdirTemp("", "admiral-backend-helm-*")
	if err != nil {
		return nil, fmt.Errorf("helm backend: create tempdir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	if err := extractArchive(archivePath, "tar.gz", dir); err != nil {
		cleanup()
		return nil, fmt.Errorf("helm backend: extract chart: %w", err)
	}

	return &FetchResult{
		Dir:      dir,
		Revision: chosen.Version,
		Digest:   chosen.Digest,
		Cleanup:  cleanup,
	}, nil
}

// loadIndex fetches and parses index.yaml, returning the parsed index and
// resolved chart name from the Source's config.
func (b *helmBackend) loadIndex(ctx context.Context, cred *model.Credential, src *model.Source) (*helmIndex, string, error) {
	target, err := helmTarget(src)
	if err != nil {
		return nil, "", err
	}
	if src.SourceConfig.Helm == nil || src.SourceConfig.Helm.ChartName == "" {
		return nil, "", fmt.Errorf("helm backend: source is missing source_config.helm.chart_name")
	}
	chartName := src.SourceConfig.Helm.ChartName
	if err := ValidateTargetURL(target); err != nil {
		return nil, "", err
	}

	indexURL := strings.TrimSuffix(target, "/") + "/index.yaml"
	resp, err := httpGetAuthed(ctx, cred, indexURL, supportBasicAndBearer)
	if err != nil {
		return nil, "", fmt.Errorf("fetch index.yaml: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("fetch %s: status %d", indexURL, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read index.yaml: %w", err)
	}

	var idx helmIndex
	if err := yaml.Unmarshal(body, &idx); err != nil {
		return nil, "", fmt.Errorf("parse index.yaml: %w", err)
	}
	return &idx, chartName, nil
}

func helmTarget(src *model.Source) (string, error) {
	if src == nil || src.URL == "" {
		return "", fmt.Errorf("source URL is required")
	}
	return src.URL, nil
}

// =============================================================================
// Shared HTTP probe helpers (used by helm, oci, terraform, http backends)
// =============================================================================

// credSupport describes a backend's accepted credential types for its HTTP
// probe. Passed to httpProbeWithCred / applyCredentialAuth.
type credSupport int

const (
	supportBasicAndBearer credSupport = iota
	supportBearerOnly
	supportBasicAndBearerAndAnon
)

// httpProbeWithCred issues an HTTP GET with credential-appropriate auth and
// maps the status. Fails fast if the credential type isn't in the backend's
// accepted set.
func httpProbeWithCred(ctx context.Context, cred *model.Credential, probeURL string, support credSupport) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
	if err != nil {
		return fmt.Errorf("build probe request: %w", err)
	}
	req.Header.Set("User-Agent", "admiral-backend/1")

	if err := applyCredentialAuth(req, cred, support); err != nil {
		return err
	}

	resp, err := NewProbeClient().Do(req)
	if err != nil {
		return fmt.Errorf("probe %s: %w", probeURL, err)
	}
	defer resp.Body.Close()
	return mapProbeStatus(resp.StatusCode, probeURL)
}
