package handler

import (
	"encoding/json"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/tagokoro/tag-judge/backend/internal/apperror"
	"github.com/tagokoro/tag-judge/backend/internal/auth"
	"github.com/tagokoro/tag-judge/backend/internal/usecase"
)

const (
	oauthCSRFCookieName = "tag_judge_oauth_csrf"
	// oauthLoginPath starts Google sign-in; next brings the user back to the
	// consent page afterwards.
	oauthLoginPath       = "/api/v1/auth/google/login"
	maxOAuthRegisterBody = 64 << 10
)

// OAuthHandler serves the OAuth 2.1 authorization server and the metadata an
// MCP client reads to find it (RFC 9728, RFC 8414).
type OAuthHandler struct {
	oauth        *usecase.OAuthUsecase
	issuer       string
	resource     string
	cookieSecure bool
}

// NewOAuthHandler takes the public origin users and clients reach Tag Judge
// at, such as https://tag-judge.example; the MCP endpoint is its /mcp.
func NewOAuthHandler(oauth *usecase.OAuthUsecase, publicBaseURL string, cookieSecure bool) *OAuthHandler {
	return &OAuthHandler{
		oauth:        oauth,
		issuer:       publicBaseURL,
		resource:     publicBaseURL + "/mcp",
		cookieSecure: cookieSecure,
	}
}

// ResourceMetadataURL is where /mcp's 401 points a client to start OAuth.
func ResourceMetadataURL(publicBaseURL string) string {
	return publicBaseURL + "/.well-known/oauth-protected-resource"
}

func (h *OAuthHandler) ProtectedResourceMetadata(w http.ResponseWriter, _ *http.Request) {
	writeOAuthJSON(w, http.StatusOK, map[string]any{
		"resource":                 h.resource,
		"authorization_servers":    []string{h.issuer},
		"bearer_methods_supported": []string{"header"},
		"resource_name":            "Tag Judge",
	})
}

func (h *OAuthHandler) AuthorizationServerMetadata(w http.ResponseWriter, _ *http.Request) {
	writeOAuthJSON(w, http.StatusOK, map[string]any{
		"issuer":                                         h.issuer,
		"authorization_endpoint":                         h.issuer + "/oauth/authorize",
		"token_endpoint":                                 h.issuer + "/oauth/token",
		"registration_endpoint":                          h.issuer + "/oauth/register",
		"response_types_supported":                       []string{"code"},
		"grant_types_supported":                          []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":               []string{"S256"},
		"token_endpoint_auth_methods_supported":          []string{usecase.OAuthAuthMethodNone, usecase.OAuthAuthMethodSecretPost, usecase.OAuthAuthMethodSecretBasic},
		"authorization_response_iss_parameter_supported": true,
	})
}

type registerOAuthClientRequest struct {
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
}

// Register is dynamic client registration (RFC 7591). Unknown metadata, such
// as logo_uri, is accepted and ignored.
func (h *OAuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerOAuthClientRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxOAuthRegisterBody)).Decode(&req); err != nil {
		writeOAuthError(w, r, apperror.InvalidInput("invalid_client_metadata", "request body must be a JSON object"))
		return
	}
	client, err := h.oauth.RegisterClient(r.Context(), usecase.RegisterOAuthClientInput{
		Name:                    req.ClientName,
		RedirectURIs:            req.RedirectURIs,
		TokenEndpointAuthMethod: req.TokenEndpointAuthMethod,
		GrantTypes:              req.GrantTypes,
		ResponseTypes:           req.ResponseTypes,
	})
	if err != nil {
		writeOAuthError(w, r, err)
		return
	}
	body := map[string]any{
		"client_id":                  client.ID,
		"client_id_issued_at":        client.IssuedAt.Unix(),
		"client_name":                client.Name,
		"redirect_uris":              client.RedirectURIs,
		"token_endpoint_auth_method": client.TokenEndpointAuthMethod,
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
	}
	if client.Secret != "" {
		body["client_secret"] = client.Secret
		body["client_secret_expires_at"] = 0
	}
	writeOAuthJSON(w, http.StatusCreated, body)
}

// Authorize shows the consent page, after sending a signed-out user through
// Google sign-in.
func (h *OAuthHandler) Authorize(w http.ResponseWriter, r *http.Request) {
	req := authorizeRequestFrom(r.URL.Query())
	client, err := h.oauth.CheckClient(r.Context(), req.ClientID, req.RedirectURI)
	if err != nil {
		h.renderError(w, r, err)
		return
	}
	if err := h.oauth.CheckRequest(req); err != nil {
		h.redirectError(w, r, req, err)
		return
	}
	if actor, ok := auth.ActorFromContext(r.Context()); !ok || actor.Method != auth.MethodSession {
		http.Redirect(w, r, oauthLoginPath+"?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
		return
	}

	csrf, err := auth.NewToken()
	if err != nil {
		h.renderError(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     oauthCSRFCookieName,
		Value:    csrf,
		Path:     "/oauth",
		MaxAge:   int((10 * time.Minute).Seconds()),
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
	redirectHost := req.RedirectURI
	if parsed, err := url.Parse(req.RedirectURI); err == nil {
		redirectHost = parsed.Host
	}
	h.renderPage(w, http.StatusOK, consentTemplate, map[string]any{
		"ClientName":   client.Name,
		"RedirectHost": redirectHost,
		"CSRF":         csrf,
		"Request":      req,
	})
}

// Decide receives the consent form and sends the user back to the client
// with a code, or with access_denied.
func (h *OAuthHandler) Decide(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.renderError(w, r, apperror.InvalidInput("invalid_request", "form is invalid"))
		return
	}
	req := authorizeRequestFrom(r.PostForm)
	if _, err := h.oauth.CheckClient(r.Context(), req.ClientID, req.RedirectURI); err != nil {
		h.renderError(w, r, err)
		return
	}
	cookie, err := r.Cookie(oauthCSRFCookieName)
	if err != nil || cookie.Value == "" || r.PostForm.Get("csrf") != cookie.Value {
		h.renderError(w, r, apperror.InvalidInput("invalid_request", "the consent page expired; start connecting again"))
		return
	}
	http.SetCookie(w, &http.Cookie{Name: oauthCSRFCookieName, Path: "/oauth", MaxAge: -1, HttpOnly: true, Secure: h.cookieSecure, SameSite: http.SameSiteStrictMode})

	if r.PostForm.Get("decision") != "approve" {
		h.redirectError(w, r, req, apperror.InvalidInput("access_denied", "the user denied access"))
		return
	}
	actor, _ := auth.ActorFromContext(r.Context())
	code, err := h.oauth.Approve(r.Context(), actor, req)
	if err != nil {
		if errors.Is(err, apperror.ErrUnauthorized) {
			h.renderError(w, r, err)
			return
		}
		h.redirectError(w, r, req, err)
		return
	}
	h.redirect(w, r, req, url.Values{"code": {code}})
}

// Token exchanges an authorization code or a refresh token for tokens.
func (h *OAuthHandler) Token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, r, apperror.InvalidInput("invalid_request", "body must be application/x-www-form-urlencoded"))
		return
	}
	creds := usecase.OAuthClientCredentials{ID: r.PostForm.Get("client_id"), Secret: r.PostForm.Get("client_secret")}
	if id, secret, ok := r.BasicAuth(); ok {
		// RFC 6749 form-encodes the credentials inside Basic auth.
		creds.ID, _ = url.QueryUnescape(id)
		creds.Secret, _ = url.QueryUnescape(secret)
	}

	var (
		tokens *usecase.OAuthTokens
		err    error
	)
	switch r.PostForm.Get("grant_type") {
	case "authorization_code":
		tokens, err = h.oauth.ExchangeCode(r.Context(), creds, r.PostForm.Get("code"), r.PostForm.Get("redirect_uri"), r.PostForm.Get("code_verifier"))
	case "refresh_token":
		tokens, err = h.oauth.Refresh(r.Context(), creds, r.PostForm.Get("refresh_token"))
	default:
		err = apperror.InvalidInput("unsupported_grant_type", "grant_type must be authorization_code or refresh_token")
	}
	if err != nil {
		writeOAuthError(w, r, err)
		return
	}
	writeOAuthJSON(w, http.StatusOK, map[string]any{
		"access_token":  tokens.AccessToken,
		"token_type":    "Bearer",
		"expires_in":    int(tokens.ExpiresIn.Seconds()),
		"refresh_token": tokens.RefreshToken,
	})
}

func authorizeRequestFrom(values url.Values) usecase.OAuthAuthorizeRequest {
	return usecase.OAuthAuthorizeRequest{
		ClientID:            values.Get("client_id"),
		RedirectURI:         values.Get("redirect_uri"),
		ResponseType:        values.Get("response_type"),
		State:               values.Get("state"),
		CodeChallenge:       values.Get("code_challenge"),
		CodeChallengeMethod: values.Get("code_challenge_method"),
		Resource:            values.Get("resource"),
	}
}

// redirectError reports err to the client. Call it only after CheckClient
// has accepted the redirect URI.
func (h *OAuthHandler) redirectError(w http.ResponseWriter, r *http.Request, req usecase.OAuthAuthorizeRequest, err error) {
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) {
		slog.ErrorContext(r.Context(), "oauth authorize failed", "error", err)
		appErr = &apperror.AppError{Code: "server_error", Message: "internal server error"}
	}
	h.redirect(w, r, req, url.Values{"error": {appErr.Code}, "error_description": {appErr.Message}})
}

func (h *OAuthHandler) redirect(w http.ResponseWriter, r *http.Request, req usecase.OAuthAuthorizeRequest, params url.Values) {
	target, err := url.Parse(req.RedirectURI)
	if err != nil {
		h.renderError(w, r, err)
		return
	}
	query := target.Query()
	for key, values := range params {
		query[key] = values
	}
	if req.State != "" {
		query.Set("state", req.State)
	}
	// iss lets the client confirm which server answered (RFC 9207).
	query.Set("iss", h.issuer)
	target.RawQuery = query.Encode()
	http.Redirect(w, r, target.String(), http.StatusFound)
}

func (h *OAuthHandler) renderError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusBadRequest
	message := "internal server error"
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		message = appErr.Message
	} else {
		slog.ErrorContext(r.Context(), "oauth authorize failed", "error", err)
		status = http.StatusInternalServerError
	}
	h.renderPage(w, status, errorTemplate, map[string]any{"Message": message})
}

func (h *OAuthHandler) renderPage(w http.ResponseWriter, status int, page *template.Template, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	// The consent page must not be framed, or another site could trick a
	// user into clicking approve.
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'")
	w.WriteHeader(status)
	_ = page.Execute(w, data)
}

func writeOAuthJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// writeOAuthError writes the error body OAuth clients expect (RFC 6749
// section 5.2) instead of the API's usual shape.
func writeOAuthError(w http.ResponseWriter, r *http.Request, err error) {
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) || appErr.Err == nil {
		slog.ErrorContext(r.Context(), "oauth request failed", "error", err)
		writeOAuthJSON(w, http.StatusInternalServerError, map[string]string{"error": "server_error", "error_description": "internal server error"})
		return
	}
	status := http.StatusBadRequest
	if errors.Is(appErr, apperror.ErrUnauthorized) {
		status = http.StatusUnauthorized
	}
	writeOAuthJSON(w, status, map[string]string{"error": appErr.Code, "error_description": appErr.Message})
}

const pageStyle = `<style>
body{margin:0;font-family:system-ui,sans-serif;background:#f6f6f4;color:#1d1d1b}
main{max-width:28rem;margin:12vh auto;padding:2rem 1.5rem;background:#fff;border:1px solid #ddd;border-radius:8px}
h1{font-size:1.25rem;margin:0 0 1rem}
p,li{line-height:1.6}
.row{display:flex;gap:.75rem;margin-top:1.5rem}
button{flex:1;padding:.6rem;font-size:1rem;border-radius:6px;border:1px solid #1d1d1b;background:#fff;cursor:pointer}
button[value=approve]{background:#1d1d1b;color:#fff}
small{color:#666}
@media (prefers-color-scheme:dark){body{background:#151514;color:#eee}main{background:#1f1f1d;border-color:#333}button{background:#1f1f1d;color:#eee;border-color:#eee}button[value=approve]{background:#eee;color:#151514}small{color:#aaa}}
</style>`

var consentTemplate = template.Must(template.New("consent").Parse(`<!doctype html>
<html lang="ja"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>アクセスの許可 - Tag Judge</title>` + pageStyle + `</head>
<body><main>
<h1>{{.ClientName}} に Tag Judge へのアクセスを許可しますか？</h1>
<p>許可すると、このアプリはあなたとして次のことができます。</p>
<ul><li>問題とテストケースの閲覧・作成・編集</li><li>コードの提出と結果の確認</li></ul>
<p><small>許可後は {{.RedirectHost}} に戻ります。心当たりがなければ拒否してください。</small></p>
<form method="post" action="/oauth/authorize">
<input type="hidden" name="csrf" value="{{.CSRF}}">
<input type="hidden" name="client_id" value="{{.Request.ClientID}}">
<input type="hidden" name="redirect_uri" value="{{.Request.RedirectURI}}">
<input type="hidden" name="response_type" value="{{.Request.ResponseType}}">
<input type="hidden" name="state" value="{{.Request.State}}">
<input type="hidden" name="code_challenge" value="{{.Request.CodeChallenge}}">
<input type="hidden" name="code_challenge_method" value="{{.Request.CodeChallengeMethod}}">
<input type="hidden" name="resource" value="{{.Request.Resource}}">
<div class="row"><button type="submit" name="decision" value="deny">拒否</button><button type="submit" name="decision" value="approve">許可</button></div>
</form>
</main></body></html>`))

var errorTemplate = template.Must(template.New("error").Parse(`<!doctype html>
<html lang="ja"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>接続できません - Tag Judge</title>` + pageStyle + `</head>
<body><main>
<h1>接続できません</h1>
<p>{{.Message}}</p>
<p><small>接続元のアプリから、もう一度接続し直してください。</small></p>
</main></body></html>`))
