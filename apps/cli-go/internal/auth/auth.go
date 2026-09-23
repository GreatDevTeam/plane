// Package auth signs in to a Plane server with an API token or an email/password pair.
//
// Plane's email/password sign-in (/auth/sign-in/) is a browser-oriented, CSRF-protected,
// redirect-based endpoint rather than a JSON API. PasswordLogin drives that flow with a
// cookie jar exactly like the web app does, then immediately exchanges the resulting
// session for a permanent API token via the session-authenticated token-creation endpoint
// (CSRF is explicitly disabled for session-authenticated REST calls on the server), so the
// rest of the client only ever has to deal with one auth mechanism: the API token.
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/makeplane/plane/apps/cli-go/internal/api"
)

// TokenLogin validates an existing API token against the server.
func TokenLogin(ctx context.Context, baseURL, token string) (*api.Client, *api.User, error) {
	c := api.New(baseURL, token)
	user, err := c.Me(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("token rejected by server: %w", err)
	}
	return c, user, nil
}

// PasswordLogin signs in with email/password, mints a fresh API token for future runs, and
// returns a ready-to-use client alongside the minted token (callers should persist it so the
// next run can use TokenLogin instead). It also returns every workspace the user belongs to,
// fetched via the session established by sign-in: that listing endpoint is session-only (no
// API-key auth), so it is only ever available right here, not from an existing token.
func PasswordLogin(ctx context.Context, baseURL, email, password string) (client *api.Client, user *api.User, token string, workspaces []api.Workspace, err error) {
	base := strings.TrimRight(baseURL, "/")

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, nil, "", nil, err
	}
	hc := &http.Client{
		Timeout: 20 * time.Second,
		Jar:     jar,
		// Sign-in responds with a redirect either way; inspect it instead of following it.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	csrfToken, err := fetchCSRFToken(ctx, hc, base)
	if err != nil {
		return nil, nil, "", nil, fmt.Errorf("fetching csrf token: %w", err)
	}

	if err := signIn(ctx, hc, base, csrfToken, email, password); err != nil {
		return nil, nil, "", nil, err
	}

	// Best-effort: an empty/nil result just means the caller falls back to asking for the
	// workspace slug by hand, same as with a bare API token.
	workspaces, _ = listWorkspaces(ctx, hc, base)

	token, err = mintAPIToken(ctx, hc, base)
	if err != nil {
		return nil, nil, "", nil, fmt.Errorf("signed in, but could not create an API token: %w", err)
	}

	client = api.New(base, token)
	user, err = client.Me(ctx)
	if err != nil {
		return nil, nil, "", nil, fmt.Errorf("signed in, but the minted token was rejected: %w", err)
	}
	return client, user, token, workspaces, nil
}

// listWorkspaces fetches every workspace the session-authenticated user belongs to via
// Plane's session-only app API (GET /api/workspaces/, the same endpoint the web app calls) —
// there is no equivalent in the API-key-authenticated public /api/v1/ surface.
func listWorkspaces(ctx context.Context, hc *http.Client, base string) ([]api.Workspace, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/workspaces/", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var workspaces []api.Workspace
	if err := json.Unmarshal(body, &workspaces); err != nil {
		return nil, err
	}
	return workspaces, nil
}

func fetchCSRFToken(ctx context.Context, hc *http.Client, base string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/auth/get-csrf-token/", nil)
	if err != nil {
		return "", err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var out struct {
		CSRFToken string `json:"csrf_token"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	if out.CSRFToken == "" {
		return "", fmt.Errorf("server did not return a csrf token")
	}
	return out.CSRFToken, nil
}

func signIn(ctx context.Context, hc *http.Client, base, csrfToken, email, password string) error {
	form := url.Values{
		"email":               {email},
		"password":            {password},
		"csrfmiddlewaretoken": {csrfToken},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/auth/sign-in/", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-CSRFToken", csrfToken)
	req.Header.Set("Referer", base)

	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusMovedPermanently {
		return fmt.Errorf("sign-in failed: unexpected response %s", resp.Status)
	}

	loc := resp.Header.Get("Location")
	locURL, err := url.Parse(loc)
	if err != nil {
		return fmt.Errorf("sign-in failed: could not parse redirect %q", loc)
	}
	if errCode := locURL.Query().Get("error_code"); errCode != "" {
		msg := locURL.Query().Get("error_message")
		if msg == "" {
			msg = errCode
		}
		return fmt.Errorf("sign-in rejected: %s", msg)
	}
	return nil
}

func mintAPIToken(ctx context.Context, hc *http.Client, base string) (string, error) {
	hostname, _ := os.Hostname()
	label := fmt.Sprintf("plane-cli-%s-%d", hostname, time.Now().Unix())
	payload, err := json.Marshal(map[string]string{
		"label":       label,
		"description": "Created by plane-cli",
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/users/api-tokens/", strings.NewReader(string(payload)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := hc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	if out.Token == "" {
		return "", fmt.Errorf("server response did not include a token")
	}
	return out.Token, nil
}
