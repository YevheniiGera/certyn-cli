package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/certyn/certyn-cli/internal/config"
)

const authScopes = "openid profile email offline_access"

type authTokenClaims struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Scope   string `json:"scope"`
	Expires int64  `json:"exp"`
}

type authTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	Error        string `json:"error"`
	Description  string `json:"error_description"`
}

type authUserInfo struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
}

func authConfigurationHint() string {
	if isNonInteractiveAuthContext() {
		return "set CERTYN_API_KEY or configure a profile api_key_ref for automation"
	}
	return "run `certyn login` or provide --api-key, CERTYN_API_KEY, or a profile api_key_ref"
}

func isNonInteractiveAuthContext() bool {
	if strings.TrimSpace(os.Getenv("CI")) != "" {
		return true
	}

	stdinInfo, err := os.Stdin.Stat()
	if err == nil && (stdinInfo.Mode()&os.ModeCharDevice) == 0 {
		return true
	}

	return false
}

func ensureBrowserAuthProfile(profileName string, profile config.Profile) config.Profile {
	if strings.TrimSpace(profile.AuthIssuer) == "" {
		profile.AuthIssuer = config.DefaultAuthIssuer
	}
	if strings.TrimSpace(profile.AccessTokenRef) == "" {
		profile.AccessTokenRef = fmt.Sprintf("%s_access_token", profileName)
	}
	if strings.TrimSpace(profile.RefreshTokenRef) == "" {
		profile.RefreshTokenRef = fmt.Sprintf("%s_refresh_token", profileName)
	}
	return profile
}

func parseAuthTokenClaims(rawToken string) (authTokenClaims, error) {
	parts := strings.Split(strings.TrimSpace(rawToken), ".")
	if len(parts) < 2 {
		return authTokenClaims{}, errors.New("token is not a JWT")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return authTokenClaims{}, fmt.Errorf("decode token payload: %w", err)
	}

	var claims authTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return authTokenClaims{}, fmt.Errorf("parse token claims: %w", err)
	}
	return claims, nil
}

func tokenIsFresh(rawToken string) bool {
	claims, err := parseAuthTokenClaims(rawToken)
	if err != nil || claims.Expires <= 0 {
		return false
	}

	return time.Now().UTC().Before(time.Unix(claims.Expires, 0).Add(-60 * time.Second))
}

func refreshBrowserSession(ctx context.Context, issuer, clientID, refreshToken string) (authTokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", strings.TrimSpace(clientID))
	form.Set("refresh_token", strings.TrimSpace(refreshToken))

	tokenURL := strings.TrimSuffix(strings.TrimSpace(issuer), "/") + "/oauth/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return authTokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "certyn-cli/"+Version)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return authTokenResponse{}, err
	}
	defer resp.Body.Close()

	var payload authTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return authTokenResponse{}, fmt.Errorf("decode token refresh response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(payload.Description)
		if message == "" {
			message = strings.TrimSpace(payload.Error)
		}
		if message == "" {
			message = fmt.Sprintf("token refresh failed with status %d", resp.StatusCode)
		}
		return authTokenResponse{}, errors.New(message)
	}

	if strings.TrimSpace(payload.AccessToken) == "" {
		return authTokenResponse{}, errors.New("token refresh returned no access token")
	}
	return payload, nil
}

func (a *App) refreshSessionIfNeeded(resolved config.Runtime) (config.Runtime, error) {
	if strings.TrimSpace(resolved.AccessToken) == "" || tokenIsFresh(resolved.AccessToken) {
		return resolved, nil
	}

	if strings.TrimSpace(resolved.RefreshToken) == "" {
		return config.Runtime{}, authError("browser session expired", errors.New("run `certyn login` again"))
	}
	if strings.TrimSpace(resolved.AuthIssuer) == "" || strings.TrimSpace(resolved.AuthClientID) == "" {
		return config.Runtime{}, authError("browser session expired", errors.New("configure auth_issuer and auth_client_id, then run `certyn login` again"))
	}

	tokenSet, err := refreshBrowserSession(context.Background(), resolved.AuthIssuer, resolved.AuthClientID, resolved.RefreshToken)
	if err != nil {
		return config.Runtime{}, authError("failed to refresh browser session", err)
	}

	resolved.AccessToken = strings.TrimSpace(tokenSet.AccessToken)
	if strings.TrimSpace(tokenSet.RefreshToken) != "" {
		resolved.RefreshToken = strings.TrimSpace(tokenSet.RefreshToken)
	}

	if a.cfg != nil {
		profile := ensureBrowserAuthProfile(resolved.Profile, a.cfg.Data.Profiles[resolved.Profile])
		if err := a.cfg.Store.Set(profile.AccessTokenRef, resolved.AccessToken); err != nil {
			return config.Runtime{}, authError("failed to persist refreshed browser session", err)
		}
		if resolved.RefreshToken != "" {
			if err := a.cfg.Store.Set(profile.RefreshTokenRef, resolved.RefreshToken); err != nil {
				return config.Runtime{}, authError("failed to persist refreshed browser session", err)
			}
		}
		a.cfg.UpsertProfile(resolved.Profile, profile)
	}

	return resolved, nil
}

func fetchUserInfo(ctx context.Context, issuer, accessToken string) (authUserInfo, error) {
	userInfoURL := strings.TrimSuffix(strings.TrimSpace(issuer), "/") + "/userinfo"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userInfoURL, nil)
	if err != nil {
		return authUserInfo{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(accessToken))
	req.Header.Set("User-Agent", "certyn-cli/"+Version)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return authUserInfo{}, err
	}
	defer resp.Body.Close()

	var info authUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return authUserInfo{}, fmt.Errorf("decode user info response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return authUserInfo{}, fmt.Errorf("userinfo failed with status %d", resp.StatusCode)
	}
	return info, nil
}
