package usecase

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

const (
	oauthCodeTTL         = 5 * time.Minute
	oauthAccessTokenTTL  = time.Hour
	oauthRefreshTokenTTL = 30 * 24 * time.Hour

	maxOAuthClientNameLength  = 100
	maxOAuthRedirectURIs      = 10
	maxOAuthRedirectURILength = 2000

	// Token prefixes let secret scanners and people tell Tag Judge
	// credentials apart.
	oauthAccessTokenPrefix  = "tja_"
	oauthRefreshTokenPrefix = "tjr_"

	OAuthAuthMethodNone        = "none"
	OAuthAuthMethodSecretPost  = "client_secret_post"
	OAuthAuthMethodSecretBasic = "client_secret_basic"
)

// OAuthUsecase is the authorization server that lets an MCP client, such as
// Claude, act for a user who approved it (OAuth 2.1 with PKCE, RFC 7591
// dynamic client registration). The errors it returns carry OAuth error codes
// such as invalid_grant, for the handler to send as they are.
type OAuthUsecase struct {
	repo repository.OAuthRepository
	// resource is the /mcp URL tokens are issued for (RFC 8707).
	resource string
	now      func() time.Time
}

func NewOAuthUsecase(repo repository.OAuthRepository, resource string) *OAuthUsecase {
	return &OAuthUsecase{repo: repo, resource: resource, now: time.Now}
}

type RegisterOAuthClientInput struct {
	Name                    string
	RedirectURIs            []string
	TokenEndpointAuthMethod string
	GrantTypes              []string
	ResponseTypes           []string
}

type RegisteredOAuthClient struct {
	ID string
	// Secret is set only for a confidential client and is returned only once.
	Secret                  string
	Name                    string
	RedirectURIs            []string
	TokenEndpointAuthMethod string
	IssuedAt                time.Time
}

func (u *OAuthUsecase) RegisterClient(ctx context.Context, input RegisterOAuthClientInput) (*RegisteredOAuthClient, error) {
	if len(input.RedirectURIs) == 0 || len(input.RedirectURIs) > maxOAuthRedirectURIs {
		return nil, oauthError("invalid_redirect_uri", "redirect_uris must list 1 to 10 URIs")
	}
	for _, uri := range input.RedirectURIs {
		if !validOAuthRedirectURI(uri) {
			return nil, oauthError("invalid_redirect_uri", "redirect_uris must be https URLs, or http URLs on a loopback host, without a fragment")
		}
	}
	for _, grant := range input.GrantTypes {
		if grant != "authorization_code" && grant != "refresh_token" {
			return nil, oauthError("invalid_client_metadata", "grant_types supports only authorization_code and refresh_token")
		}
	}
	for _, responseType := range input.ResponseTypes {
		if responseType != "code" {
			return nil, oauthError("invalid_client_metadata", "response_types supports only code")
		}
	}
	method := input.TokenEndpointAuthMethod
	if method == "" {
		method = OAuthAuthMethodNone
	}
	if method != OAuthAuthMethodNone && method != OAuthAuthMethodSecretPost && method != OAuthAuthMethodSecretBasic {
		return nil, oauthError("invalid_client_metadata", "token_endpoint_auth_method must be none, client_secret_post, or client_secret_basic")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = "MCP client"
	}
	if len([]rune(name)) > maxOAuthClientNameLength {
		return nil, oauthError("invalid_client_metadata", "client_name must be at most 100 characters")
	}

	id, err := auth.NewToken()
	if err != nil {
		return nil, err
	}
	var secret, secretHash string
	if method != OAuthAuthMethodNone {
		if secret, err = auth.NewToken(); err != nil {
			return nil, err
		}
		secretHash = auth.HashToken(secret)
	}
	if err := u.repo.CreateClient(ctx, repository.CreateOAuthClientParams{
		ID:           id,
		SecretHash:   secretHash,
		Name:         name,
		RedirectURIs: input.RedirectURIs,
	}); err != nil {
		return nil, err
	}
	return &RegisteredOAuthClient{
		ID:                      id,
		Secret:                  secret,
		Name:                    name,
		RedirectURIs:            input.RedirectURIs,
		TokenEndpointAuthMethod: method,
		IssuedAt:                u.now(),
	}, nil
}

// OAuthAuthorizeRequest is the query of an authorization request. The consent
// form carries it back unchanged.
type OAuthAuthorizeRequest struct {
	ClientID            string
	RedirectURI         string
	ResponseType        string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
	Resource            string
}

// CheckClient finds the requesting client and confirms the redirect URI is
// one it registered. When it fails, the redirect URI cannot be trusted, so
// the error must be shown to the user rather than sent there.
func (u *OAuthUsecase) CheckClient(ctx context.Context, clientID, redirectURI string) (*domain.OAuthClient, error) {
	if clientID == "" {
		return nil, oauthError("invalid_request", "client_id is required")
	}
	client, err := u.repo.FindClient(ctx, clientID)
	if errors.Is(err, apperror.ErrNotFound) {
		return nil, oauthError("invalid_client", "client is not registered")
	}
	if err != nil {
		return nil, err
	}
	if !slices.Contains(client.RedirectURIs, redirectURI) {
		return nil, oauthError("invalid_request", "redirect_uri is not registered for this client")
	}
	return client, nil
}

// CheckRequest validates the rest of an authorization request. Its errors may
// be sent to the redirect URI once CheckClient has passed.
func (u *OAuthUsecase) CheckRequest(req OAuthAuthorizeRequest) error {
	if req.ResponseType != "code" {
		return oauthError("unsupported_response_type", "response_type must be code")
	}
	if req.CodeChallengeMethod != "S256" {
		return oauthError("invalid_request", "code_challenge_method must be S256")
	}
	if n := len(req.CodeChallenge); n < 43 || n > 128 {
		return oauthError("invalid_request", "code_challenge is required")
	}
	if req.Resource != "" && !u.isResource(req.Resource) {
		return oauthError("invalid_target", "resource must be "+u.resource)
	}
	return nil
}

// Approve records the signed-in user's consent and returns the authorization
// code to send to the client. Only a browser session may approve, so a token
// cannot grant itself more access.
func (u *OAuthUsecase) Approve(ctx context.Context, actor auth.Actor, req OAuthAuthorizeRequest) (string, error) {
	if err := validateActor(actor); err != nil || actor.Method != auth.MethodSession {
		return "", apperror.Unauthorized("UNAUTHORIZED", "sign in to approve access")
	}
	if _, err := u.CheckClient(ctx, req.ClientID, req.RedirectURI); err != nil {
		return "", err
	}
	if err := u.CheckRequest(req); err != nil {
		return "", err
	}

	code, err := auth.NewToken()
	if err != nil {
		return "", err
	}
	if err := u.repo.CreateAuthorizationCode(ctx, repository.CreateOAuthAuthorizationCodeParams{
		CodeHash:      auth.HashToken(code),
		ClientID:      req.ClientID,
		UserID:        actor.UserID,
		RedirectURI:   req.RedirectURI,
		CodeChallenge: req.CodeChallenge,
		ExpiresAt:     u.now().Add(oauthCodeTTL),
	}); err != nil {
		return "", err
	}
	return code, nil
}

// OAuthClientCredentials is how a client identified itself at the token
// endpoint. Secret is empty for a public client.
type OAuthClientCredentials struct {
	ID     string
	Secret string
}

type OAuthTokens struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    time.Duration
}

func (u *OAuthUsecase) ExchangeCode(ctx context.Context, creds OAuthClientCredentials, code, redirectURI, codeVerifier string) (*OAuthTokens, error) {
	client, err := u.authenticateClient(ctx, creds)
	if err != nil {
		return nil, err
	}
	if code == "" || codeVerifier == "" {
		return nil, oauthError("invalid_request", "code and code_verifier are required")
	}
	grant, err := u.repo.ConsumeAuthorizationCode(ctx, auth.HashToken(code))
	if errors.Is(err, apperror.ErrNotFound) {
		return nil, oauthError("invalid_grant", "authorization code is invalid, used, or expired")
	}
	if err != nil {
		return nil, err
	}
	if grant.ClientID != client.ID || grant.RedirectURI != redirectURI {
		return nil, oauthError("invalid_grant", "authorization code was issued to another client or redirect_uri")
	}
	if !verifyPKCE(codeVerifier, grant.CodeChallenge) {
		return nil, oauthError("invalid_grant", "code_verifier does not match code_challenge")
	}

	tokens, params, err := u.newTokens()
	if err != nil {
		return nil, err
	}
	params.ClientID = client.ID
	params.UserID = grant.UserID
	if err := u.repo.CreateToken(ctx, params); err != nil {
		return nil, err
	}
	return tokens, nil
}

// Refresh exchanges a refresh token for a new pair. The old pair stops
// working, so a stolen refresh token is useful only until its owner refreshes.
func (u *OAuthUsecase) Refresh(ctx context.Context, creds OAuthClientCredentials, refreshToken string) (*OAuthTokens, error) {
	client, err := u.authenticateClient(ctx, creds)
	if err != nil {
		return nil, err
	}
	if refreshToken == "" {
		return nil, oauthError("invalid_request", "refresh_token is required")
	}
	tokens, params, err := u.newTokens()
	if err != nil {
		return nil, err
	}
	err = u.repo.RotateToken(ctx, client.ID, auth.HashToken(refreshToken), params)
	if errors.Is(err, apperror.ErrNotFound) {
		return nil, oauthError("invalid_grant", "refresh token is invalid, used, or expired")
	}
	if err != nil {
		return nil, err
	}
	return tokens, nil
}

func (u *OAuthUsecase) authenticateClient(ctx context.Context, creds OAuthClientCredentials) (*domain.OAuthClient, error) {
	if creds.ID == "" {
		return nil, oauthError("invalid_client", "client_id is required")
	}
	client, err := u.repo.FindClient(ctx, creds.ID)
	if errors.Is(err, apperror.ErrNotFound) {
		return nil, oauthError("invalid_client", "client is not registered")
	}
	if err != nil {
		return nil, err
	}
	if client.SecretHash != "" && subtle.ConstantTimeCompare([]byte(auth.HashToken(creds.Secret)), []byte(client.SecretHash)) != 1 {
		return nil, oauthError("invalid_client", "client authentication failed")
	}
	return client, nil
}

func (u *OAuthUsecase) newTokens() (*OAuthTokens, repository.CreateOAuthTokenParams, error) {
	access, err := auth.NewToken()
	if err != nil {
		return nil, repository.CreateOAuthTokenParams{}, err
	}
	refresh, err := auth.NewToken()
	if err != nil {
		return nil, repository.CreateOAuthTokenParams{}, err
	}
	now := u.now()
	tokens := &OAuthTokens{
		AccessToken:  oauthAccessTokenPrefix + access,
		RefreshToken: oauthRefreshTokenPrefix + refresh,
		ExpiresIn:    oauthAccessTokenTTL,
	}
	return tokens, repository.CreateOAuthTokenParams{
		AccessTokenHash:  auth.HashToken(tokens.AccessToken),
		AccessExpiresAt:  now.Add(oauthAccessTokenTTL),
		RefreshTokenHash: auth.HashToken(tokens.RefreshToken),
		RefreshExpiresAt: now.Add(oauthRefreshTokenTTL),
	}, nil
}

// isResource accepts the /mcp URL with or without a trailing slash, as
// clients normalize it differently.
func (u *OAuthUsecase) isResource(resource string) bool {
	return strings.TrimSuffix(resource, "/") == strings.TrimSuffix(u.resource, "/")
}

// validOAuthRedirectURI allows https anywhere and http only on loopback, where
// a native client such as Claude Code listens for the callback (RFC 8252).
func validOAuthRedirectURI(raw string) bool {
	if len(raw) > maxOAuthRedirectURILength {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.Fragment != "" || parsed.User != nil {
		return false
	}
	switch parsed.Scheme {
	case "https":
		return true
	case "http":
		host := parsed.Hostname()
		return host == "localhost" || host == "127.0.0.1" || host == "::1"
	}
	// Native apps such as Cursor return to their own private-use scheme
	// (RFC 8252 section 7.1). An app that hijacks the scheme gets only a
	// code, which is useless without the client's PKCE verifier.
	return !nonAppRedirectSchemes[parsed.Scheme]
}

// nonAppRedirectSchemes are schemes a browser handles itself, so a redirect
// to them never reaches an app.
var nonAppRedirectSchemes = map[string]bool{
	"about": true, "blob": true, "data": true, "file": true, "ftp": true,
	"javascript": true, "vbscript": true, "ws": true, "wss": true,
}

func verifyPKCE(verifier, challenge string) bool {
	if n := len(verifier); n < 43 || n > 128 {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	expected := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(expected), []byte(challenge)) == 1
}

// oauthError makes an error whose Code is the OAuth error code. A failed
// client authentication is a 401; everything else is a 400.
func oauthError(code, description string) *apperror.AppError {
	if code == "invalid_client" {
		return apperror.Unauthorized(code, description)
	}
	return apperror.InvalidInput(code, description)
}
