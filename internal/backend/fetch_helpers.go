package backend

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"go.admiral.io/admiral/internal/model"
)

// applyCredentialAuth mutates req to apply the credential's auth. Returns an
// error if the credential type is not in the allowed set.
func applyCredentialAuth(req *http.Request, cred *model.Credential, allowed credSupport) error {
	if cred == nil {
		return nil
	}
	switch cred.Type {
	case model.CredentialTypeBasicAuth:
		if allowed == supportBearerOnly {
			return fmt.Errorf("basic_auth is not accepted for this source type")
		}
		if cred.AuthConfig.BasicAuth == nil {
			return fmt.Errorf("credential has no basic auth configured")
		}
		req.SetBasicAuth(cred.AuthConfig.BasicAuth.Username, cred.AuthConfig.BasicAuth.Password)
	case model.CredentialTypeBearerToken:
		if cred.AuthConfig.BearerToken == nil || cred.AuthConfig.BearerToken.Token == "" {
			return fmt.Errorf("credential has no bearer token configured")
		}
		req.Header.Set("Authorization", "Bearer "+cred.AuthConfig.BearerToken.Token)
	default:
		return fmt.Errorf("credential type %s is not accepted for this source type", cred.Type)
	}
	return nil
}

// httpGetAuthed issues an authenticated GET using NewProbeClient and returns
// the response. Caller MUST close the body. Status is NOT checked here.
func httpGetAuthed(ctx context.Context, cred *model.Credential, target string, allowed credSupport) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "admiral-backend/1")
	if err := applyCredentialAuth(req, cred, allowed); err != nil {
		return nil, err
	}
	return NewProbeClient().Do(req)
}

// downloadToFile downloads the resource at absURL (with optional auth) to a
// file under tempDirPrefix, returning the file path. absURL must be an
// already-validated absolute URL.
func downloadToFile(ctx context.Context, cred *model.Credential, absURL string, allowed credSupport) (string, func(), error) {
	if err := ValidateTargetURL(absURL); err != nil {
		return "", nil, err
	}
	resp, err := httpGetAuthed(ctx, cred, absURL, allowed)
	if err != nil {
		return "", nil, fmt.Errorf("download %s: %w", absURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("download %s: unexpected status %d", absURL, resp.StatusCode)
	}

	f, err := os.CreateTemp("", "admiral-backend-dl-*")
	if err != nil {
		return "", nil, fmt.Errorf("create tempfile: %w", err)
	}
	cleanup := func() { _ = os.Remove(f.Name()) }

	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		cleanup()
		return "", nil, fmt.Errorf("download body: %w", err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("close tempfile: %w", err)
	}
	return f.Name(), cleanup, nil
}

// detectArchiveFormat returns "tar.gz", "tar", "zip", or "" based on URL
// extension. Used when Content-Type is unreliable.
func detectArchiveFormat(target string) string {
	lower := strings.ToLower(target)
	// Strip query / fragment.
	if i := strings.IndexAny(lower, "?#"); i != -1 {
		lower = lower[:i]
	}
	switch {
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return "tar.gz"
	case strings.HasSuffix(lower, ".zip"):
		return "zip"
	case strings.HasSuffix(lower, ".tar"):
		return "tar"
	}
	return ""
}

// extractArchive extracts an archive file at archivePath (format: tar.gz, tar,
// or zip) into destDir. Applies zip-slip/tar-slip guards so entries can't
// escape destDir. destDir must already exist.
func extractArchive(archivePath, format, destDir string) error {
	switch format {
	case "tar.gz", "tgz":
		return extractTarGz(archivePath, destDir)
	case "tar":
		return extractTar(archivePath, destDir)
	case "zip":
		return extractZip(archivePath, destDir)
	default:
		return fmt.Errorf("unsupported archive format %q", format)
	}
}

func extractTarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()
	return extractTarStream(gz, destDir)
}

func extractTar(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	return extractTarStream(f, destDir)
}

func extractTarStream(r io.Reader, destDir string) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}
		targetPath, err := safeJoin(destDir, hdr.Name)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil { //nolint:gosec // bounded by source tarball
				_ = out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink, tar.TypeLink:
			// Skip links for safety; they're rare in module/chart tarballs
			// and can point outside destDir in ways that are painful to
			// validate. Consumers needing symlinks can request it later.
			continue
		default:
			// Unsupported entry type; skip.
			continue
		}
	}
}

func extractZip(archivePath, destDir string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()
	for _, zf := range r.File {
		targetPath, err := safeJoin(destDir, zf.Name)
		if err != nil {
			return err
		}
		if zf.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			_ = rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil { //nolint:gosec // bounded by source archive
			_ = rc.Close()
			_ = out.Close()
			return err
		}
		_ = rc.Close()
		if err := out.Close(); err != nil {
			return err
		}
	}
	return nil
}

// safeJoin defends against zip-slip / tar-slip: rejects paths that would
// escape destDir after join + clean.
func safeJoin(destDir, entryName string) (string, error) {
	cleaned := filepath.Clean(entryName)
	if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, "/../") || strings.HasPrefix(cleaned, "/") {
		return "", fmt.Errorf("archive entry %q escapes destination", entryName)
	}
	full := filepath.Join(destDir, cleaned)
	rel, err := filepath.Rel(destDir, full)
	if err != nil {
		return "", fmt.Errorf("archive entry %q: %w", entryName, err)
	}
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("archive entry %q escapes destination", entryName)
	}
	return full, nil
}

// resolveRelativeURL resolves a possibly-relative URL against a base. Used
// when a Helm chart entry or Terraform download response gives a relative
// path.
func resolveRelativeURL(base, ref string) (string, error) {
	if strings.Contains(ref, "://") {
		return ref, nil
	}
	b, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("parse base url: %w", err)
	}
	r, err := url.Parse(ref)
	if err != nil {
		return "", fmt.Errorf("parse ref url: %w", err)
	}
	return b.ResolveReference(r).String(), nil
}

