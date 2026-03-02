package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type OAuthClient struct {
	ClientID     string   `yaml:"client_id" json:"client_id"`
	ClientSecret string   `yaml:"client_secret" json:"client_secret"`
	RedirectURIs []string `yaml:"redirect_uris" json:"redirect_uris"`
}

type authCode struct {
	code          string
	clientID      string
	customerID    int
	customerEmail string
	scope         string
	redirectURI   string
	expiresAt     time.Time
	used          bool
}

type tokenRecord struct {
	accessToken   string
	refreshToken  string
	clientID      string
	customerID    int
	customerEmail string
	scope         string
	expiresAt     time.Time
	revoked       bool
}

type OAuthServer struct {
	cfg       *Config
	clients   map[string]*OAuthClient
	codes     map[string]*authCode
	tokens    map[string]*tokenRecord
	refreshes map[string]*tokenRecord
	mu        sync.Mutex
}

func NewOAuthServer(cfg *Config, clients []OAuthClient) *OAuthServer {
	clientMap := make(map[string]*OAuthClient, len(clients))
	for i := range clients {
		clientMap[clients[i].ClientID] = &clients[i]
	}
	return &OAuthServer{
		cfg:       cfg,
		clients:   clientMap,
		codes:     make(map[string]*authCode),
		tokens:    make(map[string]*tokenRecord),
		refreshes: make(map[string]*tokenRecord),
	}
}

func (os *OAuthServer) HandleMetadata(w http.ResponseWriter, r *http.Request) {
	issuer := strings.TrimRight(os.cfg.StoreURL, "/")
	metadata := map[string]any{
		"issuer":                            issuer,
		"authorization_endpoint":            issuer + "/oauth2/authorize",
		"token_endpoint":                    issuer + "/oauth2/token",
		"revocation_endpoint":               issuer + "/oauth2/revoke",
		"scopes_supported":                  []string{"ucp:scopes:checkout_session"},
		"response_types_supported":          []string{"code"},
		"grant_types_supported":             []string{"authorization_code", "refresh_token"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_basic"},
		"service_documentation":             issuer + "/docs/oauth2",
	}
	writeJSON(w, http.StatusOK, metadata)
}

func (os *OAuthServer) HandleAuthorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	clientID := q.Get("client_id")
	redirectURI := q.Get("redirect_uri")
	responseType := q.Get("response_type")
	scope := q.Get("scope")
	state := q.Get("state")
	email := q.Get("email")

	if responseType != "code" {
		writeOAuthError(w, http.StatusBadRequest, "unsupported_response_type", "only response_type=code is supported")
		return
	}

	os.mu.Lock()
	client, ok := os.clients[clientID]
	os.mu.Unlock()
	if !ok {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client", "unknown client_id")
		return
	}

	if !uriRegistered(client.RedirectURIs, redirectURI) {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "redirect_uri not registered for this client")
		return
	}

	if email == "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "email parameter is required")
		return
	}

	code := generateRandomToken()
	ac := &authCode{
		code:          code,
		clientID:      clientID,
		customerID:    ResolveCustomerID(email),
		customerEmail: email,
		scope:         scope,
		redirectURI:   redirectURI,
		expiresAt:     time.Now().Add(10 * time.Minute),
	}

	os.mu.Lock()
	os.codes[code] = ac
	os.mu.Unlock()

	redir, err := url.Parse(redirectURI)
	if err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("malformed redirect_uri: %v", err))
		return
	}
	qv := redir.Query()
	qv.Set("code", code)
	if state != "" {
		qv.Set("state", state)
	}
	redir.RawQuery = qv.Encode()

	http.Redirect(w, r, redir.String(), http.StatusFound)
}

func (os *OAuthServer) HandleToken(w http.ResponseWriter, r *http.Request) {
	clientID, clientSecret, ok := r.BasicAuth()
	if !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="oauth"`)
		writeOAuthError(w, http.StatusUnauthorized, "invalid_client", "client authentication required")
		return
	}

	os.mu.Lock()
	client, exists := os.clients[clientID]
	os.mu.Unlock()
	if !exists || client.ClientSecret != clientSecret {
		writeOAuthError(w, http.StatusUnauthorized, "invalid_client", "invalid client credentials")
		return
	}

	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("parse form: %v", err))
		return
	}

	grantType := r.FormValue("grant_type")
	switch grantType {
	case "authorization_code":
		os.handleAuthCodeGrant(w, r, clientID)
	case "refresh_token":
		os.handleRefreshGrant(w, r, clientID)
	default:
		writeOAuthError(w, http.StatusBadRequest, "unsupported_grant_type", "supported grant_types: authorization_code, refresh_token")
	}
}

func (os *OAuthServer) handleAuthCodeGrant(w http.ResponseWriter, r *http.Request, clientID string) {
	code := r.FormValue("code")
	redirectURI := r.FormValue("redirect_uri")

	os.mu.Lock()
	ac, ok := os.codes[code]
	if !ok || ac.used || ac.clientID != clientID || time.Now().After(ac.expiresAt) {
		os.mu.Unlock()
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "authorization code is invalid or expired")
		return
	}
	if ac.redirectURI != redirectURI {
		os.mu.Unlock()
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "redirect_uri mismatch")
		return
	}
	ac.used = true
	os.mu.Unlock()

	rec := os.issueTokenPair(clientID, ac.customerID, ac.customerEmail, ac.scope)

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  rec.accessToken,
		"token_type":    "Bearer",
		"expires_in":    3600,
		"refresh_token": rec.refreshToken,
		"scope":         rec.scope,
	})
}

func (os *OAuthServer) handleRefreshGrant(w http.ResponseWriter, r *http.Request, clientID string) {
	rt := r.FormValue("refresh_token")

	os.mu.Lock()
	old, ok := os.refreshes[rt]
	if !ok || old.revoked || old.clientID != clientID || time.Now().After(old.expiresAt) {
		os.mu.Unlock()
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "refresh token is invalid or expired")
		return
	}
	old.revoked = true
	os.mu.Unlock()

	rec := os.issueTokenPair(clientID, old.customerID, old.customerEmail, old.scope)

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  rec.accessToken,
		"token_type":    "Bearer",
		"expires_in":    3600,
		"refresh_token": rec.refreshToken,
		"scope":         rec.scope,
	})
}

func (os *OAuthServer) HandleRevoke(w http.ResponseWriter, r *http.Request) {
	clientID, clientSecret, ok := r.BasicAuth()
	if !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="oauth"`)
		writeOAuthError(w, http.StatusUnauthorized, "invalid_client", "client authentication required")
		return
	}

	os.mu.Lock()
	client, exists := os.clients[clientID]
	os.mu.Unlock()
	if !exists || client.ClientSecret != clientSecret {
		writeOAuthError(w, http.StatusUnauthorized, "invalid_client", "invalid client credentials")
		return
	}

	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("parse form: %v", err))
		return
	}

	token := r.FormValue("token")
	hint := r.FormValue("token_type_hint")

	os.mu.Lock()
	defer os.mu.Unlock()

	switch hint {
	case "refresh_token":
		os.revokeRefreshTokenLocked(token)
	case "access_token":
		os.revokeAccessTokenLocked(token)
	default:
		if !os.revokeAccessTokenLocked(token) {
			os.revokeRefreshTokenLocked(token)
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (os *OAuthServer) revokeAccessTokenLocked(token string) bool {
	rec, ok := os.tokens[token]
	if !ok {
		return false
	}
	rec.revoked = true
	return true
}

func (os *OAuthServer) revokeRefreshTokenLocked(token string) bool {
	rec, ok := os.refreshes[token]
	if !ok {
		return false
	}
	rec.revoked = true
	// Revoke all access tokens for the same client+customer
	for _, tr := range os.tokens {
		if tr.clientID == rec.clientID && tr.customerEmail == rec.customerEmail {
			tr.revoked = true
		}
	}
	return true
}

func (os *OAuthServer) ValidateAccessToken(token string) (*tokenRecord, error) {
	os.mu.Lock()
	rec, ok := os.tokens[token]
	os.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("access token not found")
	}
	if rec.revoked {
		return nil, fmt.Errorf("access token revoked")
	}
	if time.Now().After(rec.expiresAt) {
		return nil, fmt.Errorf("access token expired")
	}
	return rec, nil
}

func ResolveCustomerID(email string) int {
	return 0
}

func (os *OAuthServer) issueTokenPair(clientID string, customerID int, customerEmail, scope string) *tokenRecord {
	at := generateRandomToken()
	rt := generateRandomToken()

	rec := &tokenRecord{
		accessToken:   at,
		refreshToken:  rt,
		clientID:      clientID,
		customerID:    customerID,
		customerEmail: customerEmail,
		scope:         scope,
		expiresAt:     time.Now().Add(1 * time.Hour),
	}

	rtRec := &tokenRecord{
		accessToken:   at,
		refreshToken:  rt,
		clientID:      clientID,
		customerID:    customerID,
		customerEmail: customerEmail,
		scope:         scope,
		expiresAt:     time.Now().Add(30 * 24 * time.Hour),
	}

	os.mu.Lock()
	os.tokens[at] = rec
	os.refreshes[rt] = rtRec
	os.mu.Unlock()

	return rec
}

func generateRandomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return hex.EncodeToString(b)
}

func uriRegistered(registered []string, uri string) bool {
	for _, r := range registered {
		if r == uri {
			return true
		}
	}
	return false
}

func writeOAuthError(w http.ResponseWriter, status int, errCode, description string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error":             errCode,
		"error_description": description,
	})
}
