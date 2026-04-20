package runner

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.admiral.io/admiral/internal/model"
	"go.admiral.io/admiral/internal/service/authn"
	runnerv1 "go.admiral.io/sdk/proto/admiral/runner/v1"
)

func (a *api) CreateRunnerToken(ctx context.Context, req *runnerv1.CreateRunnerTokenRequest) (*runnerv1.CreateRunnerTokenResponse, error) {
	runnerID, err := uuid.Parse(req.GetRunnerId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid runner ID: %v", err)
	}

	if _, err := a.store.Get(ctx, runnerID); err != nil {
		return nil, status.Errorf(codes.NotFound, "runner not found: %s", runnerID)
	}

	var expiry *time.Duration
	if req.ExpiresAt != nil {
		d := time.Until(req.GetExpiresAt().AsTime())
		if d <= 0 {
			return nil, status.Error(codes.InvalidArgument, "expires_at must be in the future")
		}
		expiry = &d
	}

	token, plaintext, err := a.tokenIssuer.CreateToken(
		ctx,
		authn.TokenKindSAT,
		model.AccessTokenBindingTypeRunner,
		req.GetName(),
		runnerID.String(),
		[]string{runnerExecScope},
		expiry,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			return nil, status.Errorf(codes.AlreadyExists, "token name %q already exists for this runner", req.GetName())
		}
		return nil, status.Errorf(codes.Internal, "failed to create runner token: %v", err)
	}

	return &runnerv1.CreateRunnerTokenResponse{
		AccessToken:    token.ToProto(),
		PlainTextToken: plaintext,
	}, nil
}

func (a *api) ListRunnerTokens(ctx context.Context, req *runnerv1.ListRunnerTokensRequest) (*runnerv1.ListRunnerTokensResponse, error) {
	runnerID, err := uuid.Parse(req.GetRunnerId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid runner ID: %v", err)
	}

	if _, err := a.store.Get(ctx, runnerID); err != nil {
		return nil, status.Errorf(codes.NotFound, "runner not found: %s", runnerID)
	}

	tokens, err := a.tokenStore.ListBySubject(ctx, runnerID.String(), string(model.AccessTokenKindSAT))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list runner tokens: %v", err)
	}

	resp := &runnerv1.ListRunnerTokensResponse{}
	for i := range tokens {
		resp.AccessTokens = append(resp.AccessTokens, tokens[i].ToProto())
	}
	return resp, nil
}

func (a *api) GetRunnerToken(ctx context.Context, req *runnerv1.GetRunnerTokenRequest) (*runnerv1.GetRunnerTokenResponse, error) {
	token, err := a.runnerTokenByID(ctx, req.GetTokenId())
	if err != nil {
		return nil, err
	}
	return &runnerv1.GetRunnerTokenResponse{
		AccessToken: token.ToProto(),
	}, nil
}

func (a *api) RevokeRunnerToken(ctx context.Context, req *runnerv1.RevokeRunnerTokenRequest) (*runnerv1.RevokeRunnerTokenResponse, error) {
	token, err := a.runnerTokenByID(ctx, req.GetTokenId())
	if err != nil {
		return nil, err
	}

	if err := a.tokenStore.Revoke(ctx, token.Id); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to revoke runner token: %v", err)
	}

	token.Status = model.AccessTokenStatusRevoked
	return &runnerv1.RevokeRunnerTokenResponse{
		AccessToken: token.ToProto(),
	}, nil
}

func (a *api) runnerTokenByID(ctx context.Context, tokenID string) (*model.AccessToken, error) {
	token, err := a.tokenStore.Get(ctx, tokenID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "token not found: %s", tokenID)
	}

	if token.Kind != model.AccessTokenKindSAT {
		return nil, status.Errorf(codes.NotFound, "token not found: %s", tokenID)
	}

	// Verify the parent runner still exists.
	runnerID, err := uuid.Parse(token.Subject)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "invalid runner reference on token")
	}
	if _, err := a.store.Get(ctx, runnerID); err != nil {
		return nil, status.Errorf(codes.NotFound, "runner not found: %s", runnerID)
	}

	return token, nil
}
