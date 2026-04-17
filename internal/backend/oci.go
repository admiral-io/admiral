package backend

import (
	"context"
	"fmt"

	"go.admiral.io/admiral/internal/model"
)

func init() {
	Register(model.SourceTypeOCI, func() Backend { return &ociBackend{} })
}

// ociBackend handles SOURCE_TYPE_OCI.
//
// Probe: GET {registry-root}/v2/ with basic or bearer auth.
// ListVersions / Fetch: deferred -- require OCI Distribution Spec
// implementation (manifest + blob protocol). When built, use
// oras.land/oras-go/v2 or github.com/google/go-containerregistry.
//
// Accepts BASIC_AUTH or BEARER_TOKEN.
type ociBackend struct{}

func (b *ociBackend) Probe(ctx context.Context, cred *model.Credential, src *model.Source) error {
	if src == nil || src.URL == "" {
		return fmt.Errorf("source URL is required")
	}
	if err := ValidateTargetURL(src.URL); err != nil {
		return err
	}
	root, err := ociRegistryRoot(src.URL)
	if err != nil {
		return err
	}
	return httpProbeWithCred(ctx, cred, root+"/v2/", supportBasicAndBearer)
}

func (b *ociBackend) Fetch(_ context.Context, _ *model.Credential, _ *model.Source, _ FetchOptions) (*FetchResult, error) {
	return nil, ErrOperationNotSupported
}

func (b *ociBackend) ListVersions(_ context.Context, _ *model.Credential, _ *model.Source) ([]Version, error) {
	return nil, ErrOperationNotSupported
}
