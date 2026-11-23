package router

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/domain"
	"github.com/tagokoro/tag-judge/backend/internal/repository"
)

const (
	routerBaseURL = "https://judge.example"
	// routerAccessToken is a valid access token for routerUserID as an author.
	routerAccessToken = "tja_valid-token"
	oauthRedirectURI  = "http://localhost:33418/callback"
	// oauthVerifier is a PKCE code verifier; oauthChallenge is its S256 hash.
	oauthVerifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
)

var oauthChallenge = func() string {
	sum := sha256.Sum256([]byte(oauthVerifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}()

func TestMCPChallengePointsToResourceMetadata(t *testing.T) {
	router := newTestRouter()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{}`))
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
	want := `Bearer resource_metadata="` + routerBaseURL + `/.well-known/oauth-protected-resource"`
	if got := res.Header().Get("WWW-Authenticate"); got != want {
		t.Fatalf("WWW-Authenticate = %q, want %q", got, want)
	}
}

func TestOAuthMetadata(t *testing.T) {
	router := newTestRouter()
	for _, path := range []string{"/.well-known/oauth-protected-resource", "/.well-known/oauth-protected-resource/mcp"} {
		var resource struct {
			Resource             string   `json:"resource"`
			AuthorizationServers []string `json:"authorization_servers"`
		}
		getJSON(t, router, path, &resource)
		if resource.Resource != routerBaseURL+"/mcp" || len(resource.AuthorizationServers) != 1 || resource.AuthorizationServers[0] != routerBaseURL {
			t.Fatalf("%s = %+v", path, resource)
		}
	}

	var server struct {
		Issuer                string   `json:"issuer"`
		AuthorizationEndpoint string   `json:"authorization_endpoint"`
		TokenEndpoint         string   `json:"token_endpoint"`
		RegistrationEndpoint  string   `json:"registration_endpoint"`
		CodeChallengeMethods  []string `json:"code_challenge_methods_supported"`
	}
	getJSON(t, router, "/.well-known/oauth-authorization-server", &server)
	if server.Issuer != routerBaseURL || server.AuthorizationEndpoint != routerBaseURL+"/oauth/authorize" ||
		server.TokenEndpoint != routerBaseURL+"/oauth/token" || server.RegistrationEndpoint != routerBaseURL+"/oauth/register" ||
		len(server.CodeChallengeMethods) != 1 || server.CodeChallengeMethods[0] != "S256" {
		t.Fatalf("authorization server metadata = %+v", server)
	}
}

func TestOAuthFlowConnectsMCP(t *testing.T) {
	options := newTestRouterOptions(newRouterGroupRepository(), &fakeSubmissionRepository{})
	router := newTestRouterWithHandlers(options)
	clientID := registerOAuthClient(t, router)

	// Signed out, the user is sent through Google sign-in and back.
	query := authorizeQuery(clientID)
	res := serve(router, httptest.NewRequest(http.MethodGet, "/oauth/authorize?"+query.Encode(), nil))
	if res.Code != http.StatusFound {
		t.Fatalf("signed-out authorize status = %d, want %d", res.Code, http.StatusFound)
	}
	login, _ := url.Parse(res.Header().Get("Location"))
	if login.Path != "/api/v1/auth/google/login" || !strings.HasPrefix(login.Query().Get("next"), "/oauth/authorize?") {
		t.Fatalf("signed-out authorize redirects to %q, want sign-in that returns to authorize", login)
	}

	code := approve(t, router, clientID, query)

	tokens := exchange(t, router, url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"code":          {code},
		"redirect_uri":  {oauthRedirectURI},
		"code_verifier": {oauthVerifier},
	})
	if !strings.HasPrefix(tokens.AccessToken, "tja_") || !strings.HasPrefix(tokens.RefreshToken, "tjr_") || tokens.ExpiresIn != 3600 || tokens.TokenType != "Bearer" {
		t.Fatalf("tokens = %+v", tokens)
	}

	// The code works once.
	res = postForm(router, "/oauth/token", url.Values{
		"grant_type": {"authorization_code"}, "client_id": {clientID}, "code": {code},
		"redirect_uri": {oauthRedirectURI}, "code_verifier": {oauthVerifier},
	})
	assertOAuthError(t, res, http.StatusBadRequest, "invalid_grant")

	// The access token acts for the user who approved.
	session := connectMCP(t, router, bearer(tokens.AccessToken))
	var problem struct {
		ID int64 `json:"id"`
	}
	callTool(t, session, "get_problem", map[string]any{"problem_id": 1}, &problem)

	// Refreshing rotates the pair: the old refresh token stops working.
	refreshed := exchange(t, router, url.Values{"grant_type": {"refresh_token"}, "client_id": {clientID}, "refresh_token": {tokens.RefreshToken}})
	if refreshed.AccessToken == tokens.AccessToken || refreshed.RefreshToken == tokens.RefreshToken {
		t.Fatalf("refresh returned the same tokens")
	}
	res = postForm(router, "/oauth/token", url.Values{"grant_type": {"refresh_token"}, "client_id": {clientID}, "refresh_token": {tokens.RefreshToken}})
	assertOAuthError(t, res, http.StatusBadRequest, "invalid_grant")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	if res := serve(router, req); res.Code != http.StatusUnauthorized {
		t.Fatalf("old access token status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
}

func TestOAuthTokenRejectsWrongVerifier(t *testing.T) {
	router := newTestRouter()
	clientID := registerOAuthClient(t, router)
	code := approve(t, router, clientID, authorizeQuery(clientID))

	res := postForm(router, "/oauth/token", url.Values{
		"grant_type": {"authorization_code"}, "client_id": {clientID}, "code": {code},
		"redirect_uri": {oauthRedirectURI}, "code_verifier": {strings.Repeat("x", 43)},
	})

	assertOAuthError(t, res, http.StatusBadRequest, "invalid_grant")
}

func TestOAuthAuthorizeDoesNotRedirectToUnregisteredURI(t *testing.T) {
	router := newTestRouter()
	clientID := registerOAuthClient(t, router)
	query := authorizeQuery(clientID)
	query.Set("redirect_uri", "https://evil.example/callback")
	req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?"+query.Encode(), nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "valid-session"})

	res := serve(router, req)

	if res.Code != http.StatusBadRequest || res.Header().Get("Location") != "" {
		t.Fatalf("status = %d, Location = %q, want an error page", res.Code, res.Header().Get("Location"))
	}
}

func TestOAuthAuthorizeSendsRequestErrorsToClient(t *testing.T) {
	router := newTestRouter()
	clientID := registerOAuthClient(t, router)
	query := authorizeQuery(clientID)
	query.Set("code_challenge_method", "plain")

	res := serve(router, httptest.NewRequest(http.MethodGet, "/oauth/authorize?"+query.Encode(), nil))

	location, _ := url.Parse(res.Header().Get("Location"))
	if res.Code != http.StatusFound || location.Query().Get("error") != "invalid_request" || location.Query().Get("state") != "xyz" {
		t.Fatalf("status = %d, Location = %q, want invalid_request sent to the client", res.Code, location)
	}
}

func TestOAuthConsentRequiresCSRFToken(t *testing.T) {
	router := newTestRouter()
	clientID := registerOAuthClient(t, router)
	form := authorizeQuery(clientID)
	form.Set("decision", "approve")
	form.Set("csrf", "forged")
	req := httptest.NewRequest(http.MethodPost, "/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "valid-session"})

	res := serve(router, req)

	if res.Code != http.StatusBadRequest || res.Header().Get("Location") != "" {
		t.Fatalf("status = %d, Location = %q, want the request refused", res.Code, res.Header().Get("Location"))
	}
}

func TestOAuthDenySendsAccessDenied(t *testing.T) {
	router := newTestRouter()
	clientID := registerOAuthClient(t, router)
	query := authorizeQuery(clientID)
	csrf, cookies := consentPage(t, router, query)
	query.Set("decision", "deny")
	query.Set("csrf", csrf)

	res := submitConsent(router, query, cookies)

	location, _ := url.Parse(res.Header().Get("Location"))
	if location.Query().Get("error") != "access_denied" || location.Query().Get("code") != "" {
		t.Fatalf("Location = %q, want access_denied", location)
	}
}

func TestOAuthRegisterValidatesRedirectURIs(t *testing.T) {
	router := newTestRouter()
	for _, uri := range []string{"http://evil.example/callback", "https://claude.ai/cb#frag", "javascript:alert(1)"} {
		body, _ := json.Marshal(map[string]any{"client_name": "Claude", "redirect_uris": []string{uri}})
		req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(string(body)))

		res := serve(router, req)

		assertOAuthError(t, res, http.StatusBadRequest, "invalid_redirect_uri")
	}
}

func TestOAuthConfidentialClientNeedsSecret(t *testing.T) {
	router := newTestRouter()
	req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(`{"client_name":"Claude","redirect_uris":["https://claude.ai/api/mcp/auth_callback"],"token_endpoint_auth_method":"client_secret_post"}`))
	res := serve(router, req)
	var client struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if err := json.NewDecoder(res.Body).Decode(&client); err != nil || client.ClientSecret == "" {
		t.Fatalf("register: %v, %+v", err, client)
	}

	res = postForm(router, "/oauth/token", url.Values{"grant_type": {"refresh_token"}, "client_id": {client.ClientID}, "client_secret": {"wrong"}, "refresh_token": {"tjr_x"}})

	assertOAuthError(t, res, http.StatusUnauthorized, "invalid_client")
}

func TestBearerTokenAuthenticates(t *testing.T) {
	router := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "bearer "+routerAccessToken)

	res := serve(router, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", res.Code, http.StatusOK, res.Body.String())
	}
}

func TestInvalidBearerTokenIsRejected(t *testing.T) {
	router := newTestRouter()
	// Even a public route rejects a bad token instead of serving it anonymously.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/problems", nil)
	req.Header.Set("Authorization", "Bearer tja_revoked")

	res := serve(router, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
	if got := res.Header().Get("WWW-Authenticate"); !strings.HasPrefix(got, `Bearer error="invalid_token", resource_metadata=`) {
		t.Fatalf("WWW-Authenticate = %q, want an invalid_token challenge", got)
	}
	assertErrorCode(t, res, "INVALID_TOKEN")
}

type oauthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

func registerOAuthClient(t *testing.T, router http.Handler) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/oauth/register", strings.NewReader(`{"client_name":"Claude Code","redirect_uris":["`+oauthRedirectURI+`"],"token_endpoint_auth_method":"none","logo_uri":"https://example.com/logo.png"}`))
	res := serve(router, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("register status = %d: %s", res.Code, res.Body.String())
	}
	var client struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if err := json.NewDecoder(res.Body).Decode(&client); err != nil {
		t.Fatalf("decode register: %v", err)
	}
	if client.ClientID == "" || client.ClientSecret != "" {
		t.Fatalf("public client = %+v, want an id and no secret", client)
	}
	return client.ClientID
}

func authorizeQuery(clientID string) url.Values {
	return url.Values{
		"client_id":             {clientID},
		"redirect_uri":          {oauthRedirectURI},
		"response_type":         {"code"},
		"state":                 {"xyz"},
		"code_challenge":        {oauthChallenge},
		"code_challenge_method": {"S256"},
		"resource":              {routerBaseURL + "/mcp"},
	}
}

var csrfInput = regexp.MustCompile(`name="csrf" value="([^"]+)"`)

// consentPage opens the consent page as a signed-in user and returns its CSRF
// token and the cookies to post it with.
func consentPage(t *testing.T, router http.Handler, query url.Values) (string, []*http.Cookie) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?"+query.Encode(), nil)
	session := &http.Cookie{Name: auth.SessionCookieName, Value: "valid-session"}
	req.AddCookie(session)
	res := serve(router, req)
	if res.Code != http.StatusOK {
		t.Fatalf("consent page status = %d: %s", res.Code, res.Body.String())
	}
	if res.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("consent page may be framed")
	}
	match := csrfInput.FindStringSubmatch(res.Body.String())
	if match == nil {
		t.Fatalf("consent page has no CSRF token: %s", res.Body.String())
	}
	return match[1], append(res.Result().Cookies(), session)
}

func submitConsent(router http.Handler, form url.Values, cookies []*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	return serve(router, req)
}

// approve consents as the signed-in user and returns the authorization code.
func approve(t *testing.T, router http.Handler, clientID string, query url.Values) string {
	t.Helper()
	csrf, cookies := consentPage(t, router, query)
	form := url.Values{}
	for key, values := range query {
		form[key] = values
	}
	form.Set("decision", "approve")
	form.Set("csrf", csrf)
	res := submitConsent(router, form, cookies)
	if res.Code != http.StatusFound {
		t.Fatalf("approve status = %d: %s", res.Code, res.Body.String())
	}
	location, _ := url.Parse(res.Header().Get("Location"))
	if location.Scheme+"://"+location.Host+location.Path != oauthRedirectURI || location.Query().Get("state") != "xyz" || location.Query().Get("iss") != routerBaseURL {
		t.Fatalf("approve redirects to %q", location)
	}
	code := location.Query().Get("code")
	if code == "" {
		t.Fatalf("approve redirects without a code: %q", location)
	}
	return code
}

func exchange(t *testing.T, router http.Handler, form url.Values) oauthTokenResponse {
	t.Helper()
	res := postForm(router, "/oauth/token", form)
	if res.Code != http.StatusOK {
		t.Fatalf("token status = %d: %s", res.Code, res.Body.String())
	}
	if res.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("token response may be cached")
	}
	var tokens oauthTokenResponse
	if err := json.NewDecoder(res.Body).Decode(&tokens); err != nil {
		t.Fatalf("decode tokens: %v", err)
	}
	return tokens
}

func postForm(router http.Handler, path string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return serve(router, req)
}

func serve(router http.Handler, req *http.Request) *httptest.ResponseRecorder {
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	return res
}

func getJSON(t *testing.T, router http.Handler, path string, out any) {
	t.Helper()
	res := serve(router, httptest.NewRequest(http.MethodGet, path, nil))
	if res.Code != http.StatusOK {
		t.Fatalf("%s status = %d", path, res.Code)
	}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		t.Fatalf("%s: decode: %v", path, err)
	}
}

func assertOAuthError(t *testing.T, res *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	var body struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if res.Code != status || body.Error != code {
		t.Fatalf("status = %d, error = %q, want %d %q", res.Code, body.Error, status, code)
	}
}

// fakeOAuthRepository keeps clients, codes, and tokens in memory. It starts
// with routerAccessToken issued to routerUserID.
type fakeOAuthRepository struct {
	mu      sync.Mutex
	clients map[string]domain.OAuthClient
	codes   map[string]repository.CreateOAuthAuthorizationCodeParams
	tokens  map[string]repository.CreateOAuthTokenParams // by access token hash
	revoked map[string]bool                              // by access token hash
}

func newFakeOAuthRepository() *fakeOAuthRepository {
	return &fakeOAuthRepository{
		clients: map[string]domain.OAuthClient{},
		codes:   map[string]repository.CreateOAuthAuthorizationCodeParams{},
		tokens: map[string]repository.CreateOAuthTokenParams{
			auth.HashToken(routerAccessToken): {UserID: routerUserID, AccessExpiresAt: time.Now().Add(time.Hour)},
		},
		revoked: map[string]bool{},
	}
}

func (r *fakeOAuthRepository) CreateClient(_ context.Context, params repository.CreateOAuthClientParams) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[params.ID] = domain.OAuthClient{ID: params.ID, SecretHash: params.SecretHash, Name: params.Name, RedirectURIs: params.RedirectURIs}
	return nil
}

func (r *fakeOAuthRepository) FindClient(_ context.Context, id string) (*domain.OAuthClient, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	client, ok := r.clients[id]
	if !ok {
		return nil, apperror.NotFound("OAUTH_CLIENT_NOT_FOUND", "oauth client not found")
	}
	return &client, nil
}

func (r *fakeOAuthRepository) CreateAuthorizationCode(_ context.Context, params repository.CreateOAuthAuthorizationCodeParams) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.codes[params.CodeHash] = params
	return nil
}

func (r *fakeOAuthRepository) ConsumeAuthorizationCode(_ context.Context, codeHash string) (*domain.OAuthAuthorizationCode, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	code, ok := r.codes[codeHash]
	if !ok {
		return nil, apperror.NotFound("OAUTH_CODE_NOT_FOUND", "authorization code is invalid, used, or expired")
	}
	delete(r.codes, codeHash)
	return &domain.OAuthAuthorizationCode{ClientID: code.ClientID, UserID: code.UserID, RedirectURI: code.RedirectURI, CodeChallenge: code.CodeChallenge}, nil
}

func (r *fakeOAuthRepository) CreateToken(_ context.Context, params repository.CreateOAuthTokenParams) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tokens[params.AccessTokenHash] = params
	return nil
}

func (r *fakeOAuthRepository) RotateToken(_ context.Context, clientID, refreshTokenHash string, next repository.CreateOAuthTokenParams) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for accessHash, token := range r.tokens {
		if token.RefreshTokenHash == refreshTokenHash && token.ClientID == clientID && !r.revoked[accessHash] {
			r.revoked[accessHash] = true
			next.ClientID = clientID
			next.UserID = token.UserID
			r.tokens[next.AccessTokenHash] = next
			return nil
		}
	}
	return apperror.NotFound("OAUTH_REFRESH_TOKEN_NOT_FOUND", "refresh token is invalid, used, or expired")
}

func (r *fakeOAuthRepository) FindActorByAccessTokenHash(_ context.Context, tokenHash string) (auth.Actor, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	token, ok := r.tokens[tokenHash]
	if !ok || r.revoked[tokenHash] || time.Now().After(token.AccessExpiresAt) {
		return auth.Actor{}, apperror.Unauthorized("INVALID_TOKEN", "access token is invalid, expired, or revoked")
	}
	return auth.Actor{UserID: token.UserID, Role: domain.UserRoleAuthor}, nil
}
