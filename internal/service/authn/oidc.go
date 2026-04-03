package authn

import (
	"context"
	"crypto/subtle"
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
	"go.uber.org/zap"
	"golang.org/x/oauth2"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/model"
	"go.admiral.io/admiral/internal/store"
)

const admiralProviderName = "admiral"

type OIDCProvider struct {
	httpClient        *http.Client
	oauth2            *oauth2.Config
	oidcProviderName  string
	oidcProvider      *oidc.Provider
	oidcVerifier      *oidc.IDTokenVerifier
	signingKey        string
	subjectClaim      string
	sessionRefreshTTL time.Duration
	tokenStore        *store.AuthnTokenStore
	userStore         *store.UserStore
	logger            *zap.Logger
}

func NewOIDCProvider(cfg *config.Config, logger *zap.Logger, tokens *store.AuthnTokenStore, users *store.UserStore) (Service, error) {
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
		httpClient:        httpClient,
		oauth2:            oauthConfig,
		oidcProviderName:  cfg.Services.Authn.Issuer,
		oidcProvider:      oidcProvider,
		oidcVerifier:      oidcVerifier,
		signingKey:        cfg.Services.Authn.SigningSecret,
		subjectClaim:      cfg.Services.Authn.SubjectClaim,
		sessionRefreshTTL: cfg.Services.Authn.SessionRefreshTTL,
		tokenStore:        tokens,
		userStore:         users,
		logger:            logger,
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
	token, err := jwt.ParseWithClaims(state, claims, func(token *jwt.Token) (interface{}, error) {
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

func (p *OIDCProvider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.httpClient)

	oidcToken, err := p.oauth2.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange auth code: %w", err)
	}

	oidcClaims, err := p.claimsFromOIDCToken(ctx, uuid.New(), oidcToken)
	if err != nil {
		return nil, fmt.Errorf("failed to extract claims from token: %w", err)
	}

	authenticatedUser, err := p.userStore.UpsertByProviderSubject(ctx, oidcClaims.ExternalSubject, claimsToUserInfo(oidcClaims))
	if err != nil {
		return nil, fmt.Errorf("failed to sync user: %w", err)
	}

	externalTokenId, err := uuid.Parse(oidcClaims.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token ID: %w", err)
	}

	externalToken, err := p.tokenStore.Save(ctx, externalTokenId, nil, "", oidcClaims.Subject, p.oidcProviderName, model.AuthnTokenKindExternal, oidcToken)
	if err != nil {
		return nil, fmt.Errorf("failed to store provider token: %w", err)
	}

	internalClaims := p.createInternalClaims(authenticatedUser.Id.String(), oidcClaims, ptrTimeOrZero(externalToken.ExpiresAt))
	internalToken, err := p.issueToken(internalClaims)
	if err != nil {
		return nil, fmt.Errorf("failed to issue token: %w", err)
	}

	internalTokenId, err := uuid.Parse(internalClaims.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token ID: %w", err)
	}

	_, err = p.tokenStore.Save(ctx, internalTokenId, &externalToken.Id, "", authenticatedUser.Id.String(), admiralProviderName, model.AuthnTokenKindUser, internalToken)
	if err != nil {
		return nil, fmt.Errorf("failed to store internal token: %w", err)
	}

	return internalToken, nil
}

func (p *OIDCProvider) Verify(ctx context.Context, rawToken string) (*Claims, error) {
	claims, err := p.parseTokenClaims(rawToken)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	tokenId, err := uuid.Parse(claims.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token ID: %w", err)
	}

	storedAuthnToken, err := p.tokenStore.Get(ctx, tokenId)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve token: %w", err)
	}

	if subtle.ConstantTimeCompare(storedAuthnToken.AccessToken, []byte(rawToken)) != 1 {
		return nil, errors.New("token mismatch: provided token doesn't match stored authn token")
	}

	return claims, nil
}

func (p *OIDCProvider) CreateToken(ctx context.Context, kind TokenKind, name string, subject string, scopes []string, expiry *time.Duration) (*model.AuthnToken, *oauth2.Token, error) {
	if subject == "" {
		return nil, nil, errors.New("subject is empty")
	}

	if !ValidTokenKind(string(kind)) {
		return nil, nil, fmt.Errorf("invalid token kind: %q", kind)
	}

	if kind == TokenKindSession {
		return nil, nil, errors.New("session tokens are created via the OAuth2 flow, not CreateToken")
	}

	if len(scopes) == 0 {
		return nil, nil, errors.New("scopes cannot be empty")
	}

	if err := ValidateScopes(scopes); err != nil {
		return nil, nil, err
	}

	if expiry != nil && *expiry <= 0 {
		return nil, nil, errors.New("expiry must be positive")
	}

	var dbKind model.AuthnTokenKind
	switch kind {
	case TokenKindPAT:
		dbKind = model.AuthnTokenKindUser
	case TokenKindAGT:
		dbKind = model.AuthnTokenKindAgent
	default:
		return nil, nil, fmt.Errorf("unsupported token kind for CreateToken: %q", kind)
	}

	tokenId := uuid.New()
	now := time.Now().UTC()
	claims := &Claims{
		RegisteredClaims: &jwt.RegisteredClaims{
			ID:        tokenId.String(),
			Issuer:    admiralProviderName,
			Subject:   subject,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
		Kind:   string(kind),
		Scopes: scopes,
	}
	if expiry != nil {
		claims.ExpiresAt = jwt.NewNumericDate(now.Add(*expiry))
	}

	token, err := p.issueToken(claims)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to issue token: %w", err)
	}

	authnToken, err := p.tokenStore.Save(ctx, tokenId, nil, name, subject, admiralProviderName, dbKind, token)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to store token: %w", err)
	}

	return authnToken, token, nil
}

func (p *OIDCProvider) RefreshToken(ctx context.Context, tokenID uuid.UUID) (*oauth2.Token, error) {
	aat, err := p.tokenStore.Get(ctx, tokenID)
	if err != nil {
		return nil, fmt.Errorf("failed to get stored token: %w", err)
	}

	if aat.ParentID == nil {
		return nil, errors.New("token has no parent — only session tokens can be refreshed")
	}

	// Follow parent_id to the external IdP token.
	externalToken, err := p.tokenStore.Get(ctx, *aat.ParentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get external token: %w", err)
	}

	providerTokenId := externalToken.Id

	// Attempt upstream IdP refresh if the external token is expired.
	if pt := externalToken.ToOAuth2Token(); !pt.Valid() {
		p.logger.Info("refreshing upstream IdP token", zap.String("id", providerTokenId.String()))

		httpCtx := context.WithValue(ctx, oauth2.HTTPClient, p.httpClient)
		oidcToken, err := p.oauth2.TokenSource(httpCtx, pt).Token()
		if err != nil {
			return nil, fmt.Errorf("upstream IdP refresh failed: %w", err)
		}

		oidcClaims, err := p.claimsFromOIDCToken(ctx, providerTokenId, oidcToken)
		if err != nil {
			return nil, fmt.Errorf("failed to extract claims from refreshed token: %w", err)
		}

		_, err = p.userStore.UpsertByProviderSubject(ctx, oidcClaims.ExternalSubject, claimsToUserInfo(oidcClaims))
		if err != nil {
			return nil, fmt.Errorf("failed to sync user: %w", err)
		}

		externalToken, err = p.tokenStore.Save(ctx, providerTokenId, nil, "", oidcClaims.Subject, p.oidcProviderName, model.AuthnTokenKindExternal, oidcToken)
		if err != nil {
			return nil, fmt.Errorf("failed to store refreshed external token: %w", err)
		}
	}

	// Build new internal claims from the (possibly refreshed) external token.
	oidcClaims, err := p.claimsFromOIDCToken(ctx, providerTokenId, externalToken.ToOAuth2Token())
	if err != nil {
		return nil, fmt.Errorf("failed to extract claims from token: %w", err)
	}

	internalClaims := p.createInternalClaims(aat.Subject, oidcClaims, ptrTimeOrZero(externalToken.ExpiresAt))

	internalTokenId, err := uuid.Parse(internalClaims.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token ID: %w", err)
	}

	internalToken, err := p.issueToken(internalClaims)
	if err != nil {
		return nil, fmt.Errorf("failed to issue token: %w", err)
	}

	_, err = p.tokenStore.Save(ctx, internalTokenId, &providerTokenId, "", aat.Subject, admiralProviderName, model.AuthnTokenKindUser, internalToken)
	if err != nil {
		return nil, fmt.Errorf("failed to store refreshed internal token: %w", err)
	}

	// Revoke the old internal token.
	_ = p.tokenStore.Delete(ctx, tokenID)

	return internalToken, nil
}

func (p *OIDCProvider) RevokeToken(ctx context.Context, token *oauth2.Token) error {
	claims, err := p.parseTokenClaims(token.AccessToken)
	if err != nil {
		return err
	}

	jti, err := uuid.Parse(claims.ID)
	if err != nil {
		return fmt.Errorf("failed to parse token ID: %w", err)
	}

	return p.tokenStore.Delete(ctx, jti)
}

func (p *OIDCProvider) RevokeAllTokens(ctx context.Context, subject string) (int64, error) {
	return p.tokenStore.DeleteBySubject(ctx, subject)
}

func (p *OIDCProvider) issueToken(claims *Claims) (*oauth2.Token, error) {
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(p.signingKey))
	if err != nil {
		return nil, err
	}

	tok := &oauth2.Token{
		AccessToken: accessToken,
		TokenType:   "Bearer",
	}
	if claims.ExpiresAt != nil {
		tok.Expiry = claims.ExpiresAt.Time
	}
	return tok, nil
}

func (p *OIDCProvider) parseTokenClaims(rawToken string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(rawToken, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok || token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("invalid signing method: expected HS256")
		}
		return []byte(p.signingKey), nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token claims: %w", err)
	}
	return claims, nil
}

func (p *OIDCProvider) claimsFromOIDCToken(ctx context.Context, id uuid.UUID, token *oauth2.Token) (*Claims, error) {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, errors.New("id_token was not present or invalid in oauth token")
	}

	idToken, err := p.oidcVerifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify ID token: %w", err)
	}

	oidcClaims := &Claims{}
	if err := idToken.Claims(oidcClaims); err != nil {
		return nil, fmt.Errorf("failed to parse OIDC claims: %w", err)
	}
	if oidcClaims.Email == "" {
		return nil, errors.New("required field 'email' missing from OIDC claims")
	}

	// Extract the provider subject using the configured claim.
	providerSubject, err := p.extractSubjectClaim(idToken)
	if err != nil {
		return nil, err
	}

	claims := &Claims{
		RegisteredClaims: &jwt.RegisteredClaims{
			ID:        id.String(),
			Subject:   providerSubject,
			ExpiresAt: oidcClaims.ExpiresAt,
			IssuedAt:  oidcClaims.IssuedAt,
			Issuer:    oidcClaims.Issuer,
		},
		ExternalSubject: providerSubject,
		Email:           oidcClaims.Email,
		EmailVerified:   oidcClaims.EmailVerified,
		Name:            oidcClaims.Name,
		GivenName:       oidcClaims.GivenName,
		FamilyName:      oidcClaims.FamilyName,
		Picture:         oidcClaims.Picture,
		Groups:          oidcClaims.Groups,
	}

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

func (p *OIDCProvider) createInternalClaims(subject string, oidcClaims *Claims, expiry time.Time) *Claims {
	maxExpiry := time.Now().Add(p.sessionRefreshTTL)
	if expiry.IsZero() || expiry.After(maxExpiry) {
		expiry = maxExpiry
	}

	tokenClaims := Claims{
		RegisteredClaims: &jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Subject:   subject,
			ExpiresAt: jwt.NewNumericDate(expiry),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    admiralProviderName,
		},
		ExternalSubject: oidcClaims.ExternalSubject,
		Kind:            string(TokenKindSession),
		Email:           oidcClaims.Email,
		EmailVerified:   oidcClaims.EmailVerified,
		Name:            oidcClaims.Name,
		GivenName:       oidcClaims.GivenName,
		FamilyName:      oidcClaims.FamilyName,
		Picture:         oidcClaims.Picture,
		Groups:          make([]string, len(oidcClaims.Groups)),
		Scopes:          SessionScopes,
	}

	copy(tokenClaims.Groups, oidcClaims.Groups)

	return &tokenClaims
}

func claimsToUserInfo(claims *Claims) model.UserInfo {
	return model.UserInfo{
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
		Name:          claims.Name,
		GivenName:     claims.GivenName,
		FamilyName:    claims.FamilyName,
		PictureUrl:    claims.Picture,
	}
}

func ptrTimeOrZero(t *time.Time) time.Time {
	if t != nil {
		return *t
	}
	return time.Time{}
}
