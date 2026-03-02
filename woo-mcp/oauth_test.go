package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// mockWPServer creates a mock WordPress + WooCommerce backend that handles:
// - GET /wp-json/wp/v2/users/me (WP auth validation via Basic Auth)
// - GET /wp-json/wc/v3/customers?email= (WC customer lookup)
func mockWPServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/wp-json/wp/v2/users/me":
			user, pass, ok := r.BasicAuth()
			if !ok || user != "buyer@example.com" || pass != "correct-password" {
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"code": "invalid_credentials"})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"id": 7})

		case r.URL.Path == "/wp-json/wc/v3/customers":
			email := r.URL.Query().Get("email")
			if email == "buyer@example.com" {
				json.NewEncoder(w).Encode([]map[string]any{
					{"id": 42, "email": "buyer@example.com", "first_name": "Test", "last_name": "Buyer"},
				})
			} else {
				json.NewEncoder(w).Encode([]map[string]any{})
			}

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func testOAuthServer(t *testing.T) (*OAuthServer, *Config, *httptest.Server) {
	t.Helper()
	wp := mockWPServer(t)
	wc := NewWooClient(wp.URL, "ck_test", "cs_test")
	cfg := &Config{
		StoreURL:   "https://store.example.com",
		ServerName: "TestStore",
		OAuthClients: []OAuthClient{
			{
				ClientID:     "test-client",
				ClientSecret: "test-secret",
				RedirectURIs: []string{"https://platform.example.com/callback"},
			},
		},
	}
	oauth := NewOAuthServer(cfg, cfg.OAuthClients, wc)
	return oauth, cfg, wp
}

func TestOAuthMetadata(t *testing.T) {
	oauth, _, wp := testOAuthServer(t)
	defer wp.Close()

	req := httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil)
	w := httptest.NewRecorder()
	oauth.HandleMetadata(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var meta map[string]any
	if err := json.NewDecoder(w.Body).Decode(&meta); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}

	if meta["issuer"] != "https://store.example.com" {
		t.Errorf("unexpected issuer: %v", meta["issuer"])
	}
	if meta["authorization_endpoint"] != "https://store.example.com/oauth2/authorize" {
		t.Errorf("unexpected authorization_endpoint: %v", meta["authorization_endpoint"])
	}
	if meta["token_endpoint"] != "https://store.example.com/oauth2/token" {
		t.Errorf("unexpected token_endpoint: %v", meta["token_endpoint"])
	}
	if meta["revocation_endpoint"] != "https://store.example.com/oauth2/revoke" {
		t.Errorf("unexpected revocation_endpoint: %v", meta["revocation_endpoint"])
	}

	scopes, ok := meta["scopes_supported"].([]any)
	if !ok || len(scopes) != 1 || scopes[0] != "ucp:scopes:checkout_session" {
		t.Errorf("unexpected scopes_supported: %v", meta["scopes_supported"])
	}

	grantTypes, ok := meta["grant_types_supported"].([]any)
	if !ok || len(grantTypes) != 2 {
		t.Errorf("unexpected grant_types_supported: %v", meta["grant_types_supported"])
	}
}

func TestOAuthAuthorizeFormRendered(t *testing.T) {
	oauth, _, wp := testOAuthServer(t)
	defer wp.Close()

	q := url.Values{
		"client_id":     {"test-client"},
		"redirect_uri":  {"https://platform.example.com/callback"},
		"response_type": {"code"},
		"scope":         {"ucp:scopes:checkout_session"},
	}
	req := httptest.NewRequest("GET", "/oauth2/authorize?"+q.Encode(), nil)
	w := httptest.NewRecorder()
	oauth.HandleAuthorize(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected text/html content type, got %s", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Sign In") {
		t.Error("login form should contain 'Sign In'")
	}
	if !strings.Contains(body, `name="email"`) {
		t.Error("login form should contain email input")
	}
	if !strings.Contains(body, `name="password"`) {
		t.Error("login form should contain password input")
	}
}

func TestOAuthAuthorizeSuccess(t *testing.T) {
	oauth, _, wp := testOAuthServer(t)
	defer wp.Close()

	form := url.Values{
		"client_id":     {"test-client"},
		"redirect_uri":  {"https://platform.example.com/callback"},
		"response_type": {"code"},
		"scope":         {"ucp:scopes:checkout_session"},
		"state":         {"xyz123"},
		"email":         {"buyer@example.com"},
		"password":      {"correct-password"},
	}
	req := httptest.NewRequest("POST", "/oauth2/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	oauth.HandleAuthorize(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d: %s", w.Code, w.Body.String())
	}

	loc, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}

	if loc.Scheme+"://"+loc.Host+loc.Path != "https://platform.example.com/callback" {
		t.Errorf("unexpected redirect base: %s", loc.String())
	}
	if loc.Query().Get("code") == "" {
		t.Error("missing code in redirect")
	}
	if loc.Query().Get("state") != "xyz123" {
		t.Errorf("unexpected state: %s", loc.Query().Get("state"))
	}
}

func TestOAuthAuthorizeInvalidPassword(t *testing.T) {
	oauth, _, wp := testOAuthServer(t)
	defer wp.Close()

	form := url.Values{
		"client_id":     {"test-client"},
		"redirect_uri":  {"https://platform.example.com/callback"},
		"response_type": {"code"},
		"email":         {"buyer@example.com"},
		"password":      {"wrong-password"},
	}
	req := httptest.NewRequest("POST", "/oauth2/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	oauth.HandleAuthorize(w, req)

	// Should re-render the login form with error message (200 with HTML)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (re-rendered form), got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Invalid email or password") {
		t.Error("should show error message for invalid credentials")
	}
}

func TestOAuthAuthorizeMissingCredentials(t *testing.T) {
	oauth, _, wp := testOAuthServer(t)
	defer wp.Close()

	form := url.Values{
		"client_id":     {"test-client"},
		"redirect_uri":  {"https://platform.example.com/callback"},
		"response_type": {"code"},
		"email":         {"buyer@example.com"},
		// password missing
	}
	req := httptest.NewRequest("POST", "/oauth2/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	oauth.HandleAuthorize(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (re-rendered form), got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Email and password are required") {
		t.Error("should show error message for missing credentials")
	}
}

func TestOAuthAuthorizeInvalidClient(t *testing.T) {
	oauth, _, wp := testOAuthServer(t)
	defer wp.Close()

	q := url.Values{
		"client_id":     {"unknown-client"},
		"redirect_uri":  {"https://platform.example.com/callback"},
		"response_type": {"code"},
	}
	req := httptest.NewRequest("GET", "/oauth2/authorize?"+q.Encode(), nil)
	w := httptest.NewRecorder()
	oauth.HandleAuthorize(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestOAuthAuthorizeInvalidRedirectURI(t *testing.T) {
	oauth, _, wp := testOAuthServer(t)
	defer wp.Close()

	q := url.Values{
		"client_id":     {"test-client"},
		"redirect_uri":  {"https://evil.example.com/callback"},
		"response_type": {"code"},
	}
	req := httptest.NewRequest("GET", "/oauth2/authorize?"+q.Encode(), nil)
	w := httptest.NewRecorder()
	oauth.HandleAuthorize(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestOAuthAuthorizeUnsupportedResponseType(t *testing.T) {
	oauth, _, wp := testOAuthServer(t)
	defer wp.Close()

	q := url.Values{
		"client_id":     {"test-client"},
		"redirect_uri":  {"https://platform.example.com/callback"},
		"response_type": {"token"},
	}
	req := httptest.NewRequest("GET", "/oauth2/authorize?"+q.Encode(), nil)
	w := httptest.NewRecorder()
	oauth.HandleAuthorize(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func doAuthorize(t *testing.T, oauth *OAuthServer) string {
	t.Helper()
	form := url.Values{
		"client_id":     {"test-client"},
		"redirect_uri":  {"https://platform.example.com/callback"},
		"response_type": {"code"},
		"scope":         {"ucp:scopes:checkout_session"},
		"state":         {"s"},
		"email":         {"buyer@example.com"},
		"password":      {"correct-password"},
	}
	req := httptest.NewRequest("POST", "/oauth2/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	oauth.HandleAuthorize(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("authorize failed: %d: %s", w.Code, w.Body.String())
	}
	loc, _ := url.Parse(w.Header().Get("Location"))
	return loc.Query().Get("code")
}

func doTokenExchange(t *testing.T, oauth *OAuthServer, code string) (string, string) {
	t.Helper()
	form := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {"https://platform.example.com/callback"},
	}
	req := httptest.NewRequest("POST", "/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("test-client", "test-secret")
	w := httptest.NewRecorder()
	oauth.HandleToken(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("token exchange failed: %d %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	return resp["access_token"].(string), resp["refresh_token"].(string)
}

func TestOAuthTokenExchange(t *testing.T) {
	oauth, _, wp := testOAuthServer(t)
	defer wp.Close()

	code := doAuthorize(t, oauth)
	at, rt := doTokenExchange(t, oauth, code)

	if at == "" {
		t.Error("empty access_token")
	}
	if rt == "" {
		t.Error("empty refresh_token")
	}

	// Validate access token
	rec, err := oauth.ValidateAccessToken(at)
	if err != nil {
		t.Fatalf("validate access token: %v", err)
	}
	if rec.customerEmail != "buyer@example.com" {
		t.Errorf("unexpected email: %s", rec.customerEmail)
	}
	if rec.scope != "ucp:scopes:checkout_session" {
		t.Errorf("unexpected scope: %s", rec.scope)
	}
	// Customer ID should be resolved from mock WC API
	if rec.customerID != 42 {
		t.Errorf("expected customerID 42 (from WC lookup), got %d", rec.customerID)
	}
}

func TestOAuthTokenExchangeInvalidCode(t *testing.T) {
	oauth, _, wp := testOAuthServer(t)
	defer wp.Close()

	form := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {"invalid-code"},
		"redirect_uri": {"https://platform.example.com/callback"},
	}
	req := httptest.NewRequest("POST", "/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("test-client", "test-secret")
	w := httptest.NewRecorder()
	oauth.HandleToken(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestOAuthTokenExchangeWrongClient(t *testing.T) {
	oauth, _, wp := testOAuthServer(t)
	defer wp.Close()

	form := url.Values{
		"grant_type": {"authorization_code"},
		"code":       {"x"},
	}
	req := httptest.NewRequest("POST", "/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("test-client", "wrong-secret")
	w := httptest.NewRecorder()
	oauth.HandleToken(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestOAuthTokenExchangeNoAuth(t *testing.T) {
	oauth, _, wp := testOAuthServer(t)
	defer wp.Close()

	form := url.Values{"grant_type": {"authorization_code"}}
	req := httptest.NewRequest("POST", "/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	oauth.HandleToken(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestOAuthCodeReuse(t *testing.T) {
	oauth, _, wp := testOAuthServer(t)
	defer wp.Close()

	code := doAuthorize(t, oauth)

	// First exchange succeeds
	doTokenExchange(t, oauth, code)

	// Second exchange with same code fails
	form := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {"https://platform.example.com/callback"},
	}
	req := httptest.NewRequest("POST", "/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("test-client", "test-secret")
	w := httptest.NewRecorder()
	oauth.HandleToken(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on code reuse, got %d", w.Code)
	}
}

func TestOAuthRefreshToken(t *testing.T) {
	oauth, _, wp := testOAuthServer(t)
	defer wp.Close()

	code := doAuthorize(t, oauth)
	_, rt := doTokenExchange(t, oauth, code)

	// Use refresh token
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {rt},
	}
	req := httptest.NewRequest("POST", "/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("test-client", "test-secret")
	w := httptest.NewRecorder()
	oauth.HandleToken(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("refresh failed: %d %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	newAT := resp["access_token"].(string)
	newRT := resp["refresh_token"].(string)

	if newAT == "" || newRT == "" {
		t.Fatal("empty tokens from refresh")
	}

	// New access token is valid
	rec, err := oauth.ValidateAccessToken(newAT)
	if err != nil {
		t.Fatalf("validate new access token: %v", err)
	}
	if rec.customerEmail != "buyer@example.com" {
		t.Errorf("unexpected email: %s", rec.customerEmail)
	}

	// Old refresh token should be revoked
	form2 := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {rt},
	}
	req2 := httptest.NewRequest("POST", "/oauth2/token", strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.SetBasicAuth("test-client", "test-secret")
	w2 := httptest.NewRecorder()
	oauth.HandleToken(w2, req2)

	if w2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on reused refresh token, got %d", w2.Code)
	}
}

func TestOAuthRevoke(t *testing.T) {
	oauth, _, wp := testOAuthServer(t)
	defer wp.Close()

	code := doAuthorize(t, oauth)
	at, _ := doTokenExchange(t, oauth, code)

	// Revoke access token
	form := url.Values{
		"token":           {at},
		"token_type_hint": {"access_token"},
	}
	req := httptest.NewRequest("POST", "/oauth2/revoke", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("test-client", "test-secret")
	w := httptest.NewRecorder()
	oauth.HandleRevoke(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("revoke failed: %d", w.Code)
	}

	// Access token no longer valid
	_, err := oauth.ValidateAccessToken(at)
	if err == nil {
		t.Fatal("expected error for revoked token")
	}
}

func TestOAuthRevokeRefreshCascades(t *testing.T) {
	oauth, _, wp := testOAuthServer(t)
	defer wp.Close()

	code := doAuthorize(t, oauth)
	at, rt := doTokenExchange(t, oauth, code)

	// Revoke refresh token — should also revoke access tokens
	form := url.Values{
		"token":           {rt},
		"token_type_hint": {"refresh_token"},
	}
	req := httptest.NewRequest("POST", "/oauth2/revoke", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("test-client", "test-secret")
	w := httptest.NewRecorder()
	oauth.HandleRevoke(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("revoke failed: %d", w.Code)
	}

	// Access token should be revoked too
	_, err := oauth.ValidateAccessToken(at)
	if err == nil {
		t.Fatal("expected access token to be revoked after refresh token revocation")
	}
}

func TestOAuthRevokeUnknownToken(t *testing.T) {
	oauth, _, wp := testOAuthServer(t)
	defer wp.Close()

	form := url.Values{"token": {"nonexistent-token"}}
	req := httptest.NewRequest("POST", "/oauth2/revoke", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("test-client", "test-secret")
	w := httptest.NewRecorder()
	oauth.HandleRevoke(w, req)

	// RFC 7009: always 200 even for unknown tokens
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for unknown token, got %d", w.Code)
	}
}

func TestOAuthFullLifecycle(t *testing.T) {
	oauth, cfg, wp := testOAuthServer(t)
	defer wp.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", oauth.HandleMetadata)
	mux.HandleFunc("GET /oauth2/authorize", oauth.HandleAuthorize)
	mux.HandleFunc("POST /oauth2/authorize", oauth.HandleAuthorize)
	mux.HandleFunc("POST /oauth2/token", oauth.HandleToken)
	mux.HandleFunc("POST /oauth2/revoke", oauth.HandleRevoke)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// 1. Discover metadata
	resp, err := http.Get(ts.URL + "/.well-known/oauth-authorization-server")
	if err != nil {
		t.Fatalf("metadata request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("metadata status: %d", resp.StatusCode)
	}
	var meta map[string]any
	json.NewDecoder(resp.Body).Decode(&meta)
	if meta["issuer"] != cfg.StoreURL {
		t.Errorf("unexpected issuer: %v", meta["issuer"])
	}

	// 2. GET authorize renders login form
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	authURL := ts.URL + "/oauth2/authorize?" + url.Values{
		"client_id":     {"test-client"},
		"redirect_uri":  {"https://platform.example.com/callback"},
		"response_type": {"code"},
		"scope":         {"ucp:scopes:checkout_session"},
	}.Encode()

	resp2, err := client.Get(authURL)
	if err != nil {
		t.Fatalf("authorize GET request: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("authorize GET status: %d (expected 200 login form)", resp2.StatusCode)
	}

	// 3. POST credentials to authorize
	authForm := url.Values{
		"client_id":     {"test-client"},
		"redirect_uri":  {"https://platform.example.com/callback"},
		"response_type": {"code"},
		"scope":         {"ucp:scopes:checkout_session"},
		"state":         {"abc"},
		"email":         {"buyer@example.com"},
		"password":      {"correct-password"},
	}
	resp3, err := client.PostForm(ts.URL+"/oauth2/authorize", authForm)
	if err != nil {
		t.Fatalf("authorize POST request: %v", err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusFound {
		t.Fatalf("authorize POST status: %d", resp3.StatusCode)
	}
	loc, _ := url.Parse(resp3.Header.Get("Location"))
	code := loc.Query().Get("code")
	if code == "" {
		t.Fatal("no code in redirect")
	}

	// 4. Exchange code for token
	tokenForm := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {"https://platform.example.com/callback"},
	}
	tokenReq, _ := http.NewRequest("POST", ts.URL+"/oauth2/token", strings.NewReader(tokenForm.Encode()))
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenReq.SetBasicAuth("test-client", "test-secret")
	resp4, err := http.DefaultClient.Do(tokenReq)
	if err != nil {
		t.Fatalf("token request: %v", err)
	}
	defer resp4.Body.Close()
	if resp4.StatusCode != http.StatusOK {
		t.Fatalf("token status: %d", resp4.StatusCode)
	}
	var tokenResp map[string]any
	json.NewDecoder(resp4.Body).Decode(&tokenResp)
	at := tokenResp["access_token"].(string)
	rt := tokenResp["refresh_token"].(string)

	// 5. Validate token
	rec, err := oauth.ValidateAccessToken(at)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if rec.customerEmail != "buyer@example.com" {
		t.Errorf("unexpected email: %s", rec.customerEmail)
	}
	if rec.customerID != 42 {
		t.Errorf("expected customerID 42, got %d", rec.customerID)
	}

	// 6. Refresh
	refreshForm := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {rt},
	}
	refreshReq, _ := http.NewRequest("POST", ts.URL+"/oauth2/token", strings.NewReader(refreshForm.Encode()))
	refreshReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	refreshReq.SetBasicAuth("test-client", "test-secret")
	resp5, err := http.DefaultClient.Do(refreshReq)
	if err != nil {
		t.Fatalf("refresh request: %v", err)
	}
	defer resp5.Body.Close()
	if resp5.StatusCode != http.StatusOK {
		t.Fatalf("refresh status: %d", resp5.StatusCode)
	}
	var refreshResp map[string]any
	json.NewDecoder(resp5.Body).Decode(&refreshResp)
	newAT := refreshResp["access_token"].(string)

	// 7. Revoke
	revokeForm := url.Values{"token": {newAT}, "token_type_hint": {"access_token"}}
	revokeReq, _ := http.NewRequest("POST", ts.URL+"/oauth2/revoke", strings.NewReader(revokeForm.Encode()))
	revokeReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	revokeReq.SetBasicAuth("test-client", "test-secret")
	resp6, err := http.DefaultClient.Do(revokeReq)
	if err != nil {
		t.Fatalf("revoke request: %v", err)
	}
	defer resp6.Body.Close()
	if resp6.StatusCode != http.StatusOK {
		t.Fatalf("revoke status: %d", resp6.StatusCode)
	}

	// Verify revoked
	_, err = oauth.ValidateAccessToken(newAT)
	if err == nil {
		t.Fatal("expected error for revoked token")
	}
}

func TestExtractAuthenticatedEmail(t *testing.T) {
	oauth, cfg, wp := testOAuthServer(t)
	defer wp.Close()

	wc := NewWooClient("http://localhost", "ck", "cs")
	rs := NewRESTServer(wc, cfg, oauth)

	// Get a valid token
	code := doAuthorize(t, oauth)
	at, _ := doTokenExchange(t, oauth, code)

	// With valid bearer token
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+at)
	email := rs.extractAuthenticatedEmail(req)
	if email != "buyer@example.com" {
		t.Errorf("expected buyer@example.com, got %q", email)
	}

	// Without token
	req2 := httptest.NewRequest("GET", "/", nil)
	email2 := rs.extractAuthenticatedEmail(req2)
	if email2 != "" {
		t.Errorf("expected empty email, got %q", email2)
	}

	// With invalid token
	req3 := httptest.NewRequest("GET", "/", nil)
	req3.Header.Set("Authorization", "Bearer invalid-token")
	email3 := rs.extractAuthenticatedEmail(req3)
	if email3 != "" {
		t.Errorf("expected empty email for invalid token, got %q", email3)
	}

	// Without OAuth server
	rsNoOAuth := NewRESTServer(wc, cfg, nil)
	req4 := httptest.NewRequest("GET", "/", nil)
	req4.Header.Set("Authorization", "Bearer "+at)
	email4 := rsNoOAuth.extractAuthenticatedEmail(req4)
	if email4 != "" {
		t.Errorf("expected empty email without oauth server, got %q", email4)
	}
}

func TestUCPProfileIncludesIdentityLinking(t *testing.T) {
	profile := NewDefaultUCPProfile("https://store.example.com")
	found := false
	for _, cap := range profile.UCP.Capabilities {
		if cap.Name == "dev.ucp.common.identity_linking" {
			found = true
			if cap.Version != "2026-01-11" {
				t.Errorf("unexpected version: %s", cap.Version)
			}
			break
		}
	}
	if !found {
		t.Error("identity_linking capability not found in UCP profile")
	}
}
