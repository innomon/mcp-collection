package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func generateTestKeyPair(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return priv, &priv.PublicKey
}

func createSignedToken(t *testing.T, priv *rsa.PrivateKey, claims jwt.Claims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	s, err := token.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestVerifyTokenValid(t *testing.T) {
	priv, pub := generateTestKeyPair(t)
	tokenStr := createSignedToken(t, priv, jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})

	tok, err := VerifyToken(tokenStr, pub)
	if err != nil {
		t.Fatalf("expected valid token, got error: %v", err)
	}
	if !tok.Valid {
		t.Fatal("token should be valid")
	}
}

func TestVerifyTokenExpired(t *testing.T) {
	priv, pub := generateTestKeyPair(t)
	tokenStr := createSignedToken(t, priv, jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
	})

	_, err := VerifyToken(tokenStr, pub)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestVerifyTokenWrongMethod(t *testing.T) {
	secret := []byte("test-secret")
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	tokenStr, err := token.SignedString(secret)
	if err != nil {
		t.Fatal(err)
	}

	_, pub := generateTestKeyPair(t)
	_, err = VerifyToken(tokenStr, pub)
	if err == nil {
		t.Fatal("expected error for wrong signing method")
	}
}

func TestVerifyTokenMalformed(t *testing.T) {
	_, pub := generateTestKeyPair(t)
	_, err := VerifyToken("not-a-jwt", pub)
	if err == nil {
		t.Fatal("expected error for malformed token")
	}
}

func TestLoadPublicKey(t *testing.T) {
	_, pub := generateTestKeyPair(t)
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "pub.pem")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := pem.Encode(f, &pem.Block{Type: "PUBLIC KEY", Bytes: der}); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	loaded, err := LoadPublicKey(path)
	if err != nil {
		t.Fatalf("expected to load key, got error: %v", err)
	}
	if !pub.Equal(loaded) {
		t.Fatal("loaded key does not match original")
	}
}

func TestLoadPublicKeyMissingFile(t *testing.T) {
	_, err := LoadPublicKey("/nonexistent/path/key.pem")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestAuthenticatedHandlerValid(t *testing.T) {
	priv, pub := generateTestKeyPair(t)
	tokenStr := createSignedToken(t, priv, jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})

	called := false
	inner := func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		called = true
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "ok"}},
		}, nil
	}

	wrapped := AuthenticatedHandler(pub, inner)
	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Meta: mcp.Meta{"authorization": "Bearer " + tokenStr},
		},
	}

	res, err := wrapped(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatal("expected success result")
	}
	if !called {
		t.Fatal("inner handler was not called")
	}
}

func TestAuthenticatedHandlerInvalid(t *testing.T) {
	_, pub := generateTestKeyPair(t)

	called := false
	inner := func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		called = true
		return nil, nil
	}

	wrapped := AuthenticatedHandler(pub, inner)
	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Meta: mcp.Meta{"authorization": "Bearer invalid-token"},
		},
	}

	res, err := wrapped(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error result")
	}
	if called {
		t.Fatal("inner handler should not be called")
	}
}

func TestAuthenticatedHandlerNilKey(t *testing.T) {
	called := false
	inner := func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		called = true
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "ok"}},
		}, nil
	}

	wrapped := AuthenticatedHandler(nil, inner)
	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{},
	}

	res, err := wrapped(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Fatal("expected success result")
	}
	if !called {
		t.Fatal("inner handler should be called when pubKey is nil")
	}
}
