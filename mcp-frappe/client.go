package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type FrappeClient struct {
	Config     Config
	HTTPClient *http.Client
}

func (c *FrappeClient) Do(ctx context.Context, method, path string, body []byte) (string, error) {
	resourceURL := fmt.Sprintf("%s/api/resource/%s", strings.TrimRight(c.Config.BaseURL, "/"), path)
	req, err := http.NewRequestWithContext(ctx, method, resourceURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("token %s:%s", c.Config.APIKey, c.Config.APISecret))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("frappe error (%d): %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return string(respBody), nil
}

func (c *FrappeClient) CallMethod(ctx context.Context, method string, params url.Values) (string, error) {
	methodURL := fmt.Sprintf("%s/api/method/%s", strings.TrimRight(c.Config.BaseURL, "/"), method)
	if encoded := params.Encode(); encoded != "" {
		methodURL = methodURL + "?" + encoded
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, methodURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("token %s:%s", c.Config.APIKey, c.Config.APISecret))

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("frappe error (%d): %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return string(respBody), nil
}

func (c *FrappeClient) GetDocTypes(ctx context.Context) (any, error) {
	params := url.Values{}
	params.Set("fields", "[\"name\"]")
	params.Set("limit_page_length", "none")
	res, err := c.Do(ctx, http.MethodGet, "DocType", nil)
	if err != nil {
		return nil, err
	}
	return decodeJSONOrString(res), nil
}

func (c *FrappeClient) GetModules(ctx context.Context) (any, error) {
	params := url.Values{}
	params.Set("fields", "[\"name\"]")
	params.Set("limit_page_length", "none")
	res, err := c.Do(ctx, http.MethodGet, "Module Def", nil)
	if err != nil {
		return nil, err
	}
	return decodeJSONOrString(res), nil
}

func (c *FrappeClient) GetApps(ctx context.Context) (any, error) {
	res, err := c.CallMethod(ctx, "frappe.utils.change_log.get_versions", nil)
	if err != nil {
		return nil, err
	}
	return decodeJSONOrString(res), nil
}

func (c *FrappeClient) GetSystemInfo(ctx context.Context) (any, error) {
	// Fallback to a simple call if specific info method is missing
	res, err := c.CallMethod(ctx, "frappe.realtime.get_user_info", nil)
	if err != nil {
		// If get_user_info fails, try a generic ping or version check
		return c.GetApps(ctx)
	}
	return decodeJSONOrString(res), nil
}

func getDocTypeMeta(ctx context.Context, client *FrappeClient, docType string) (map[string]any, error) {
	path := fmt.Sprintf("DocType/%s", escapePathSegment(docType))
	res, err := client.Do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	decoded := decodeJSONOrString(res)
	meta, ok := decoded.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected metadata shape from Frappe")
	}
	return meta, nil
}

func decodeJSONOrString(raw string) any {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]any{}
	}

	var parsed any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
		return parsed
	}
	return raw
}

func escapePathSegment(value string) string {
	return url.PathEscape(strings.TrimSpace(value))
}
