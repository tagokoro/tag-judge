package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/tagokoro/tag-judge/backend/internal/usecase"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const googleUserInfoURL = "https://openidconnect.googleapis.com/v1/userinfo"

type GoogleProvider struct {
	config *oauth2.Config
}

func NewGoogleProvider(clientID, clientSecret, redirectURL string) *GoogleProvider {
	if clientID == "" || clientSecret == "" || redirectURL == "" {
		return nil
	}

	return &GoogleProvider{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
	}
}

func (p *GoogleProvider) AuthCodeURL(state string) string {
	return p.config.AuthCodeURL(state)
}

func (p *GoogleProvider) ExchangeUser(ctx context.Context, code string) (usecase.OAuthUser, error) {
	token, err := p.config.Exchange(ctx, code)
	if err != nil {
		return usecase.OAuthUser{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleUserInfoURL, nil)
	if err != nil {
		return usecase.OAuthUser{}, err
	}
	token.SetAuthHeader(req)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return usecase.OAuthUser{}, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return usecase.OAuthUser{}, fmt.Errorf("google userinfo returned status %d", res.StatusCode)
	}

	var body struct {
		Sub   string `json:"sub"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return usecase.OAuthUser{}, err
	}
	if body.Sub == "" || body.Email == "" {
		return usecase.OAuthUser{}, fmt.Errorf("google userinfo response missing required fields")
	}

	return usecase.OAuthUser{
		ProviderUserID: body.Sub,
		Name:           body.Name,
		Email:          body.Email,
	}, nil
}
