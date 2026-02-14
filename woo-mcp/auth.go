package main

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func LoadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA public key")
	}
	return rsaPub, nil
}

func VerifyToken(tokenStr string, pubKey *rsa.PublicKey) (*jwt.Token, error) {
	return jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("invalid signing method: %v", token.Header["alg"])
		}
		return pubKey, nil
	})
}

func AuthenticatedHandler(pubKey *rsa.PublicKey, handler func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error)) func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if pubKey == nil {
			return handler(ctx, req)
		}

		meta := req.Params.GetMeta()
		authVal, ok := meta["authorization"]
		if !ok {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "missing authorization"}},
				IsError: true,
			}, nil
		}

		authStr, ok := authVal.(string)
		if !ok || !strings.HasPrefix(authStr, "Bearer ") {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "invalid authorization format"}},
				IsError: true,
			}, nil
		}

		tokenStr := strings.TrimPrefix(authStr, "Bearer ")
		if _, err := VerifyToken(tokenStr, pubKey); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "authentication failed: " + err.Error()}},
				IsError: true,
			}, nil
		}

		return handler(ctx, req)
	}
}
