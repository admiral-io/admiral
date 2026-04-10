package authn

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"go.uber.org/zap"
	"golang.org/x/oauth2"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/model"
	"go.admiral.io/admiral/internal/store"
)

type OIDCProvider struct {
	httpClient       *http.Client
	oauth2           *oauth2.Config
	oidcProviderName string
	oidcProvider     *oidc.Provider
	oidcVerifier     *oidc.IDTokenVerifier
	signingKey       string
	subjectClaim     string
	tokenStore       *store.AccessTokenStore
	userStore        *store.UserStore
	logger           *zap.Logger
}

func NewOIDCProvider(cfg *config.Config, logger *zap.Logger, tokens *store.AccessTokenStore, users *store.UserStore) (Service, error) {
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: cfg.Services.Authn.SkipTLSVerify, //nolint:gosec
			},
		},
	}
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, httpClient)

	oidcProvider, err := oidc.NewProvider(ctx, cfg.Services.Authn.Issuer)
	if err != nil {
		logger.Error("failed to initialize oidc provider", zap.Error(err))
		return nil, fmt.Errorf("failed to initialize oidc provider: %w", err)
	}

	oidcVerifier := oidcProvider.Verifier(&oidc.Config{
		ClientID: cfg.Services.Authn.ClientID,
	})

	oauthConfig := &oauth2.Config{
		ClientID:     cfg.Services.Authn.ClientID,
		ClientSecret: cfg.Services.Authn.ClientSecret,
		Endpoint:     oidcProvider.Endpoint(),
		RedirectURL:  cfg.Services.Authn.RedirectURL,
		Scopes:       cfg.Services.Authn.Scopes,
	}

	return &OIDCProvider{
		httpClient:       httpClient,
		oauth2:           oauthConfig,
		oidcProviderName: cfg.Services.Authn.Issuer,
		oidcProvider:     oidcProvider,
		oidcVerifier:     oidcVerifier,
		signingKey:       cfg.Services.Authn.SigningSecret,
		subjectClaim:     cfg.Services.Authn.SubjectClaim,
		tokenStore:       tokens,
		userStore:        users,
		logger:           logger,
	}, nil
}

func (p *OIDCProvider) GetStateNonce(_ context.Context, redirectURL string) (string, error) {
	u, err := url.Parse(redirectURL)
	if err != nil {
		return "", fmt.Errorf("invalid redirect URL: %w", err)
	}

	if u.Scheme != "" || u.Host != "" {
		return "", errors.New("only relative redirect URLs are supported")
	}

	dest := u.RequestURI()
	if !strings.HasPrefix(dest, "/") {
		dest = fmt.Sprintf("/%s", dest)
	}

	claims := &stateClaims{
		RegisteredClaims: &jwt.RegisteredClaims{
			Subject:   uuid.New().String(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		RedirectURL: dest,
	}

	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(p.signingKey))
}

func (p *OIDCProvider) ValidateStateNonce(_ context.Context, state string) (string, error) {
	claims := &stateClaims{}
	token, err := jwt.ParseWithClaims(state, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok || token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("invalid signing method: expected HS256")
		}
		return []byte(p.signingKey), nil
	})
	if err != nil {
		return "", fmt.Errorf("invalid state token: %w", err)
	}

	if !token.Valid {
		return "", fmt.Errorf("state token is invalid")
	}

	if err := claims.Validate(); err != nil {
		return "", fmt.Errorf("state token validation failed: %w", err)
	}

	return claims.RedirectURL, nil
}

func (p *OIDCProvider) GetAuthCodeURL(_ context.Context, state string) (string, error) {
	if state == "" {
		return "", errors.New("state parameter cannot be empty")
	}

	authURL := p.oauth2.AuthCodeURL(state)
	if authURL == "" {
		return "", errors.New("failed to generate auth code URL")
	}

	return authURL, nil
}

func (p *OIDCProvider) Exchange(ctx context.Context, code string) (string, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.httpClient)

	oidcToken, err := p.oauth2.Exchange(ctx, code)
	if err != nil {
		return "", fmt.Errorf("failed to exchange auth code: %w", err)
	}

	oidcClaims, err := p.claimsFromOIDCToken(ctx, oidcToken)
	if err != nil {
		return "", fmt.Errorf("failed to extract claims from token: %w", err)
	}

	authenticatedUser, err := p.userStore.UpsertByProviderSubject(ctx, oidcClaims.Subject, claimsToUserInfo(oidcClaims))
	if err != nil {
		return "", fmt.Errorf("failed to sync user: %w", err)
	}

	plaintext, tokenHash, err := GenerateOpaqueToken(TokenKindSession)
	if err != nil {
		return "", fmt.Errorf("failed to generate session token: %w", err)
	}

	session := &model.AccessToken{
		Id:          uuid.NewString(),
		Subject:     authenticatedUser.Id.String(),
		Kind:        model.AccessTokenKindSession,
		TokenHash:   tokenHash,
		TokenPrefix: plaintext[:len(PrefixSession)],
		Scopes:      pq.StringArray(SessionScopes),
		Issuer:      p.oidcProviderName,
		ExpiresAt:   idpTokenExpiry(oidcToken),
	}

	if oidcToken.AccessToken != "" {
		session.IdpAccessToken = []byte(oidcToken.AccessToken)
	}
	if oidcToken.RefreshToken != "" {
		session.IdpRefreshToken = []byte(oidcToken.RefreshToken)
	}
	if it, ok := oidcToken.Extra("id_token").(string); ok && it != "" {
		session.IdpIdToken = []byte(it)
	}

	_, err = p.tokenStore.Create(ctx, session)
	if err != nil {
		return "", fmt.Errorf("failed to store session: %w", err)
	}

	return plaintext, nil
}

func (p *OIDCProvider) Verify(ctx context.Context, credential string) (*Claims, error) {
	if !IsOpaqueToken(credential) {
		return nil, errors.New("invalid credential: unrecognized token format")
	}

	if !ValidateChecksum(credential) {
		return nil, errors.New("invalid token: checksum mismatch")
	}

	hash := HashOpaqueToken(credential)

	at, err := p.tokenStore.GetByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	if at.ExpiresAt != nil && at.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("token has expired")
	}

	var kind TokenKind
	switch at.Kind {
	case model.AccessTokenKindPAT:
		kind = TokenKindPAT
	case model.AccessTokenKindSAT:
		kind = TokenKindSAT
	case model.AccessTokenKindSession:
		kind = TokenKindSession
	}

	return &Claims{
		Subject: at.Subject,
		Kind:    string(kind),
		Scopes:  at.Scopes,
	}, nil
}

func (p *OIDCProvider) RefreshSession(ctx context.Context, sessionToken string) error {
	hash := HashOpaqueToken(sessionToken)

	session, err := p.tokenStore.GetByHash(ctx, hash)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	if session.Kind != model.AccessTokenKindSession {
		return errors.New("token is not a session")
	}

	// Refresh upstream IdP token if expired.
	idpToken := session.IdPToken()
	if !idpToken.Valid() {
		p.logger.Info("refreshing upstream IdP token", zap.String("session_id", session.Id))

		httpCtx := context.WithValue(ctx, oauth2.HTTPClient, p.httpClient)
		refreshedToken, err := p.oauth2.TokenSource(httpCtx, idpToken).Token()
		if err != nil {
			return fmt.Errorf("upstream IdP refresh failed: %w", err)
		}

		idpToken = refreshedToken

		oidcClaims, err := p.claimsFromOIDCToken(ctx, idpToken)
		if err != nil {
			return fmt.Errorf("failed to extract claims from refreshed token: %w", err)
		}

		_, err = p.userStore.UpsertByProviderSubject(ctx, oidcClaims.Subject, claimsToUserInfo(oidcClaims))
		if err != nil {
			return fmt.Errorf("failed to sync user: %w", err)
		}
	}

	// Update session row in place with refreshed IdP tokens and new expiry.
	return p.tokenStore.UpdateIdPTokens(ctx, session.Id, idpToken, idpTokenExpiry(idpToken))
}

func (p *OIDCProvider) CreateToken(ctx context.Context, kind TokenKind, name string, subject string, scopes []string, expiry *time.Duration) (*model.AccessToken, string, error) {
	if subject == "" {
		return nil, "", errors.New("subject is empty")
	}

	if kind == TokenKindSession {
		return nil, "", errors.New("session tokens are created via the OAuth2 flow, not CreateToken")
	}

	if kind != TokenKindPAT && kind != TokenKindSAT {
		return nil, "", fmt.Errorf("unsupported token kind for CreateToken: %q", kind)
	}

	if len(scopes) == 0 {
		return nil, "", errors.New("scopes cannot be empty")
	}

	if err := ValidateScopes(scopes); err != nil {
		return nil, "", err
	}

	if expiry != nil && *expiry <= 0 {
		return nil, "", errors.New("expiry must be positive")
	}

	plaintext, tokenHash, err := GenerateOpaqueToken(kind)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate token: %w", err)
	}

	dbKind := model.AccessTokenKindPAT
	if kind == TokenKindSAT {
		dbKind = model.AccessTokenKindSAT
	}

	at := &model.AccessToken{
		Id:          uuid.NewString(),
		Name:        name,
		Subject:     subject,
		Kind:        dbKind,
		TokenHash:   tokenHash,
		TokenPrefix: plaintext[:len(PrefixPAT)],
		Scopes:      pq.StringArray(scopes),
	}

	if expiry != nil {
		exp := time.Now().UTC().Add(*expiry)
		at.ExpiresAt = &exp
	}

	saved, err := p.tokenStore.Create(ctx, at)
	if err != nil {
		return nil, "", fmt.Errorf("failed to store token: %w", err)
	}

	return saved, plaintext, nil
}

func (p *OIDCProvider) RevokeToken(ctx context.Context, rawToken string) error {
	if !IsOpaqueToken(rawToken) {
		return errors.New("invalid token: unrecognized token format")
	}

	hash := HashOpaqueToken(rawToken)

	at, err := p.tokenStore.GetByHash(ctx, hash)
	if err != nil {
		return fmt.Errorf("token not found: %w", err)
	}

	return p.tokenStore.Delete(ctx, at.Id)
}

func (p *OIDCProvider) RevokeAllTokens(ctx context.Context, subject string) (int64, error) {
	return p.tokenStore.DeleteBySubject(ctx, subject)
}

// oidcProfileClaims holds IdP token fields used transiently during login
// to sync user info. Not carried in request context.
type oidcProfileClaims struct {
	Subject       string   `json:"sub"`
	Email         string   `json:"email"`
	EmailVerified bool     `json:"email_verified"`
	Name          string   `json:"name"`
	GivenName     string   `json:"given_name"`
	FamilyName    string   `json:"family_name"`
	Picture       string   `json:"picture"`
	Groups        []string `json:"groups"`
}

func (p *OIDCProvider) claimsFromOIDCToken(ctx context.Context, token *oauth2.Token) (*oidcProfileClaims, error) {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, errors.New("id_token was not present or invalid in oauth token")
	}

	idToken, err := p.oidcVerifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify ID token: %w", err)
	}

	claims := &oidcProfileClaims{}
	if err := idToken.Claims(claims); err != nil {
		return nil, fmt.Errorf("failed to parse OIDC claims: %w", err)
	}
	if claims.Email == "" {
		return nil, errors.New("required field 'email' missing from OIDC claims")
	}

	providerSubject, err := p.extractSubjectClaim(idToken)
	if err != nil {
		return nil, err
	}
	claims.Subject = providerSubject

	return claims, nil
}

func (p *OIDCProvider) extractSubjectClaim(idToken *oidc.IDToken) (string, error) {
	if p.subjectClaim == "sub" {
		return idToken.Subject, nil
	}

	var rawClaims map[string]any
	if err := idToken.Claims(&rawClaims); err != nil {
		return "", fmt.Errorf("failed to parse raw ID token claims: %w", err)
	}

	val, ok := rawClaims[p.subjectClaim]
	if !ok {
		return "", fmt.Errorf("configured subject_claim %q not found in ID token", p.subjectClaim)
	}

	subject, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("subject_claim %q is not a string in ID token", p.subjectClaim)
	}

	if subject == "" {
		return "", fmt.Errorf("subject_claim %q is empty in ID token", p.subjectClaim)
	}

	return subject, nil
}

func idpTokenExpiry(token *oauth2.Token) *time.Time {
	if token == nil || token.Expiry.IsZero() {
		return nil
	}
	return new(token.Expiry)
}

func claimsToUserInfo(claims *oidcProfileClaims) model.UserInfo {
	return model.UserInfo{
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
		Name:          claims.Name,
		GivenName:     claims.GivenName,
		FamilyName:    claims.FamilyName,
		PictureUrl:    claims.Picture,
	}
}
