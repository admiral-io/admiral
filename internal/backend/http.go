package backend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"go.admiral.io/admiral/internal/model"
)

func init() {
	Register(model.SourceTypeHTTP, func() Backend { return &httpBackend{} })
}

// httpBackend handles SOURCE_TYPE_HTTP (bare HTTP tar/zip archive).
//
// Probe: GET {url} with configured auth.
// ListVersions: not applicable -- a single URL is a single artifact.
// Fetch: download, detect format from extension, extract into a tempdir.
// Revision is the sha256 of the downloaded archive (the only stable
// identifier for a versionless URL).
//
// Accepts BASIC_AUTH, BEARER_TOKEN, or anonymous.
type httpBackend struct{}

func (b *httpBackend) Probe(ctx context.Context, cred *model.Credential, src *model.Source) error {
	if src == nil || src.URL == "" {
		return fmt.Errorf("source URL is required")
	}
	if err := ValidateTargetURL(src.URL); err != nil {
		return err
	}
	return httpProbeWithCred(ctx, cred, src.URL, supportBasicAndBearerAndAnon)
}

func (b *httpBackend) Fetch(ctx context.Context, cred *model.Credential, src *model.Source, _ FetchOptions) (*FetchResult, error) {
	if src == nil || src.URL == "" {
		return nil, fmt.Errorf("source URL is required")
	}
	format := detectArchiveFormat(src.URL)
	if format == "" {
		return nil, fmt.Errorf("http backend: cannot detect archive format from URL %q; expected .tar.gz / .tgz / .tar / .zip suffix", src.URL)
	}

	archivePath, removeArchive, err := downloadToFile(ctx, cred, src.URL, supportBasicAndBearerAndAnon)
	if err != nil {
		return nil, err
	}
	defer removeArchive()

	digest, err := sha256File(archivePath)
	if err != nil {
		return nil, err
	}

	dir, err := os.MkdirTemp("", "admiral-backend-http-*")
	if err != nil {
		return nil, fmt.Errorf("http backend: create tempdir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	if err := extractArchive(archivePath, format, dir); err != nil {
		cleanup()
		return nil, fmt.Errorf("http backend: extract archive: %w", err)
	}

	return &FetchResult{
		Dir:      dir,
		Revision: digest,
		Digest:   "sha256:" + digest,
		Cleanup:  cleanup,
	}, nil
}

func (b *httpBackend) ListVersions(_ context.Context, _ *model.Credential, _ *model.Source) ([]Version, error) {
	return nil, ErrOperationNotSupported
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open archive for hashing: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash archive: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
