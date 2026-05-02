package backend

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	gogitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gogitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
	cryptossh "golang.org/x/crypto/ssh"

	"go.admiral.io/admiral/internal/model"
)

func init() {
	Register(model.SourceTypeGit, func() Backend { return &gitBackend{} })
}

// gitBackend handles SOURCE_TYPE_GIT.
//
// Accepted credential types:
//   - nil        → anonymous (public repos)
//   - BASIC_AUTH → HTTP Basic (GitHub/GitLab PAT stored as user+token)
//   - SSH_KEY    → SSH public key auth
type gitBackend struct{}

func (b *gitBackend) Probe(ctx context.Context, cred *model.Credential, src *model.Source) error {
	target, err := gitTarget(src)
	if err != nil {
		return err
	}
	if err := validateGitTarget(target); err != nil {
		return err
	}
	if isSSHURL(target) {
		return b.probeSSH(ctx, cred, target)
	}
	return b.probeHTTPS(ctx, cred, target)
}

func (b *gitBackend) probeHTTPS(ctx context.Context, cred *model.Credential, target string) error {
	probeURL := strings.TrimSuffix(target, "/") + "/info/refs?service=git-upload-pack"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
	if err != nil {
		return fmt.Errorf("build probe request: %w", err)
	}
	req.Header.Set("User-Agent", "admiral-backend-git/1")

	if cred != nil {
		switch cred.Type {
		case model.CredentialTypeBasicAuth:
			if cred.AuthConfig.BasicAuth == nil {
				return fmt.Errorf("credential has no basic auth configured")
			}
			req.SetBasicAuth(cred.AuthConfig.BasicAuth.Username, cred.AuthConfig.BasicAuth.Password)
		case model.CredentialTypeSSHKey:
			return fmt.Errorf("ssh_key credential cannot be used against an HTTPS git URL; use an ssh:// or git@host URL instead")
		default:
			return fmt.Errorf("git backend: unsupported credential type %s for HTTPS", cred.Type)
		}
	}

	resp, err := NewProbeClient().Do(req)
	if err != nil {
		return fmt.Errorf("probe %s: %w", target, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("authentication failed against %s (status %d)", target, resp.StatusCode)
	case http.StatusNotFound:
		return fmt.Errorf("repository not found or inaccessible: %s", target)
	default:
		return fmt.Errorf("unexpected status %d probing %s", resp.StatusCode, target)
	}
}

func (b *gitBackend) probeSSH(ctx context.Context, cred *model.Credential, target string) error {
	if cred == nil {
		return fmt.Errorf("anonymous SSH probe is not supported; an SSH_KEY credential is required")
	}
	if cred.Type != model.CredentialTypeSSHKey {
		return fmt.Errorf("credential type %s cannot be used against an SSH git URL; use SSH_KEY", cred.Type)
	}
	auth, err := gitSSHAuth(cred)
	if err != nil {
		return err
	}
	remote := gogit.NewRemote(memory.NewStorage(), &gogitconfig.RemoteConfig{
		Name: "origin",
		URLs: []string{target},
	})
	if _, err := remote.ListContext(ctx, &gogit.ListOptions{Auth: auth}); err != nil {
		return fmt.Errorf("ssh probe %s: %w", target, err)
	}
	return nil
}

func (b *gitBackend) Fetch(ctx context.Context, cred *model.Credential, src *model.Source, opts FetchOptions) (*FetchResult, error) {
	target, err := gitTarget(src)
	if err != nil {
		return nil, err
	}
	if err := validateGitTarget(target); err != nil {
		return nil, err
	}

	dir, err := os.MkdirTemp("", "admiral-backend-git-*")
	if err != nil {
		return nil, fmt.Errorf("git backend: create tempdir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	cloneOpts := &gogit.CloneOptions{URL: target, SingleBranch: true}
	auth, err := fetchAuth(cred, target)
	if err != nil {
		cleanup()
		return nil, err
	}
	cloneOpts.Auth = auth

	var postCheckoutSHA string
	if opts.Ref != "" {
		if isLikelyCommitSHA(opts.Ref) {
			postCheckoutSHA = opts.Ref
		} else {
			cloneOpts.ReferenceName = plumbing.NewBranchReferenceName(opts.Ref)
		}
	}

	repo, err := gogit.PlainCloneContext(ctx, dir, false, cloneOpts)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("git backend: clone %s: %w", target, err)
	}

	if postCheckoutSHA != "" {
		wt, err := repo.Worktree()
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("git backend: worktree: %w", err)
		}
		if err := wt.Checkout(&gogit.CheckoutOptions{Hash: plumbing.NewHash(postCheckoutSHA)}); err != nil {
			cleanup()
			return nil, fmt.Errorf("git backend: checkout %s: %w", postCheckoutSHA, err)
		}
	}

	head, err := repo.Head()
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("git backend: resolve HEAD: %w", err)
	}
	revision := head.Hash().String()

	if opts.Root != "" {
		rootDir := filepath.Join(dir, opts.Root)
		info, err := os.Stat(rootDir)
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("git backend: subtree %q not found: %w", opts.Root, err)
		}
		if !info.IsDir() {
			cleanup()
			return nil, fmt.Errorf("git backend: subtree %q is not a directory", opts.Root)
		}
	}

	return &FetchResult{
		Dir:              dir,
		WorkingDirectory: opts.Root,
		Revision:         revision,
		Digest:           "sha1:" + revision,
		Cleanup:          cleanup,
	}, nil
}

func (b *gitBackend) ListVersions(ctx context.Context, cred *model.Credential, src *model.Source) ([]Version, error) {
	target, err := gitTarget(src)
	if err != nil {
		return nil, err
	}
	if err := validateGitTarget(target); err != nil {
		return nil, err
	}

	auth, err := fetchAuth(cred, target)
	if err != nil {
		return nil, err
	}

	remote := gogit.NewRemote(memory.NewStorage(), &gogitconfig.RemoteConfig{
		Name: "origin",
		URLs: []string{target},
	})
	refs, err := remote.ListContext(ctx, &gogit.ListOptions{Auth: auth})
	if err != nil {
		return nil, fmt.Errorf("list refs: %w", err)
	}

	var versions []Version
	for _, ref := range refs {
		if ref.Name().IsTag() {
			versions = append(versions, Version{Name: ref.Name().Short()})
		}
	}
	return versions, nil
}

// gitTarget extracts the URL from a Source.
func gitTarget(src *model.Source) (string, error) {
	if src == nil || src.URL == "" {
		return "", fmt.Errorf("source URL is required")
	}
	return src.URL, nil
}

// validateGitTarget runs SSRF / host-block validation for a git URL.
func validateGitTarget(target string) error {
	if isSSHURL(target) {
		host, err := sshHost(target)
		if err != nil {
			return err
		}
		return ValidateHostNotBlocked(host)
	}
	return ValidateTargetURL(target)
}

// fetchAuth builds a go-git AuthMethod from a credential.
func fetchAuth(cred *model.Credential, target string) (transport.AuthMethod, error) {
	if cred == nil {
		return nil, nil
	}
	switch cred.Type {
	case model.CredentialTypeBasicAuth:
		if isSSHURL(target) {
			return nil, fmt.Errorf("basic_auth credential cannot be used against an SSH git URL")
		}
		if cred.AuthConfig.BasicAuth == nil {
			return nil, fmt.Errorf("credential has no basic auth configured")
		}
		return &githttp.BasicAuth{
			Username: cred.AuthConfig.BasicAuth.Username,
			Password: cred.AuthConfig.BasicAuth.Password,
		}, nil
	case model.CredentialTypeSSHKey:
		return gitSSHAuth(cred)
	default:
		return nil, fmt.Errorf("git backend: unsupported credential type %s", cred.Type)
	}
}

func gitSSHAuth(cred *model.Credential) (*gogitssh.PublicKeys, error) {
	if cred.AuthConfig.SSHKey == nil || cred.AuthConfig.SSHKey.PrivateKey == "" {
		return nil, fmt.Errorf("credential has no ssh key configured")
	}
	auth, err := gogitssh.NewPublicKeys("git",
		[]byte(cred.AuthConfig.SSHKey.PrivateKey),
		cred.AuthConfig.SSHKey.Passphrase,
	)
	if err != nil {
		return nil, fmt.Errorf("parse ssh private key: %w", err)
	}
	auth.HostKeyCallback = cryptossh.InsecureIgnoreHostKey() //nolint:gosec // documented trade-off
	return auth, nil
}

func isSSHURL(target string) bool {
	if strings.HasPrefix(target, "ssh://") {
		return true
	}
	if strings.Contains(target, "://") {
		return false
	}
	at := strings.Index(target, "@")
	colon := strings.Index(target, ":")
	return at != -1 && colon > at
}

func sshHost(target string) (string, error) {
	ep, err := transport.NewEndpoint(target)
	if err != nil {
		return "", fmt.Errorf("invalid SSH target %q: %w", target, err)
	}
	if ep.Host == "" {
		return "", fmt.Errorf("SSH target %q has no host", target)
	}
	return ep.Host, nil
}

func isLikelyCommitSHA(ref string) bool {
	if len(ref) < 7 || len(ref) > 40 {
		return false
	}
	for _, r := range ref {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}
