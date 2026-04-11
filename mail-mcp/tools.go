package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterTools(s *mcp.Server, cfg *Config) {
	// IMAP Tools
	s.AddTool(&mcp.Tool{
		Name:        "list_folders",
		Description: "List all mail folders (INBOX, Sent, etc.)",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"account_id":{"type":"string","description":"Optional account ID (defaults to first account)"}}}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			AccountID string `json:"account_id"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return nil, fmt.Errorf("parsing arguments: %w", err)
		}

		acc, err := cfg.GetAccount(args.AccountID)
		if err != nil {
			return nil, fmt.Errorf("getting account: %w", err)
		}

		client := NewMailClient(acc)
		folders, err := client.ListFolders(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing folders: %w", err)
		}

		var lines []string
		for _, f := range folders {
			lines = append(lines, fmt.Sprintf("%s (Attrs: %v)", f.Name, f.Attributes))
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: strings.Join(lines, "\n")}},
		}, nil
	})

	s.AddTool(&mcp.Tool{
		Name:        "list_messages",
		Description: "List recent messages in a folder",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"folder":{"type":"string","description":"Folder name (default: INBOX)"},"limit":{"type":"integer","description":"Number of messages to fetch (default 10)"},"account_id":{"type":"string"}},"required":["folder"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			Folder    string `json:"folder"`
			Limit     int    `json:"limit"`
			AccountID string `json:"account_id"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return nil, fmt.Errorf("parsing arguments: %w", err)
		}

		if args.Limit <= 0 {
			args.Limit = 10
		}

		acc, err := cfg.GetAccount(args.AccountID)
		if err != nil {
			return nil, fmt.Errorf("getting account: %w", err)
		}

		client := NewMailClient(acc)
		headers, err := client.ListMessages(ctx, args.Folder, args.Limit)
		if err != nil {
			return nil, fmt.Errorf("listing messages: %w", err)
		}

		var lines []string
		for _, h := range headers {
			lines = append(lines, fmt.Sprintf("UID: %d | From: %s | Subject: %s | Date: %s", h.UID, h.From, h.Subject, h.Date))
		}

		if len(lines) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "No messages found."}},
			}, nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: strings.Join(lines, "\n")}},
		}, nil
	})

	s.AddTool(&mcp.Tool{
		Name:        "get_message",
		Description: "Fetch full message content",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"uid":{"type":"string","description":"Message UID"},"folder":{"type":"string"},"account_id":{"type":"string"}},"required":["uid","folder"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			UID       string `json:"uid"`
			Folder    string `json:"folder"`
			AccountID string `json:"account_id"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return nil, fmt.Errorf("parsing arguments: %w", err)
		}

		uid, err := strconv.ParseUint(args.UID, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid UID %q: %w", args.UID, err)
		}

		acc, err := cfg.GetAccount(args.AccountID)
		if err != nil {
			return nil, fmt.Errorf("getting account: %w", err)
		}

		client := NewMailClient(acc)
		detail, err := client.GetMessage(ctx, args.Folder, uint32(uid))
		if err != nil {
			return nil, fmt.Errorf("getting message %d: %w", uid, err)
		}

		res, err := json.MarshalIndent(detail, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshaling result: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(res)}},
		}, nil
	})

	// SMTP Tools
	s.AddTool(&mcp.Tool{
		Name:        "send_email",
		Description: "Send a new email",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"to":{"type":"array","items":{"type":"string"}},
				"subject":{"type":"string"},
				"body":{"type":"string"},
				"is_html":{"type":"boolean"},
				"attachments":{
					"type":"array",
					"items":{
						"type":"object",
						"properties":{
							"filename":{"type":"string"},
							"content":{"type":"string","description":"Base64 encoded content"}
						},
						"required":["filename","content"]
					}
				},
				"account_id":{"type":"string"}
			},
			"required":["to","subject","body"]
		}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			To          []string `json:"to"`
			Subject     string   `json:"subject"`
			Body        string   `json:"body"`
			IsHTML      bool     `json:"is_html"`
			Attachments []struct {
				Filename string `json:"filename"`
				Content  string `json:"content"`
			} `json:"attachments"`
			AccountID string `json:"account_id"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return nil, fmt.Errorf("parsing arguments: %w", err)
		}

		acc, err := cfg.GetAccount(args.AccountID)
		if err != nil {
			return nil, fmt.Errorf("getting account: %w", err)
		}

		var sendAtts []SendAttachment
		for _, a := range args.Attachments {
			data, err := base64.StdEncoding.DecodeString(a.Content)
			if err != nil {
				return nil, fmt.Errorf("decoding attachment %q: %w", a.Filename, err)
			}
			sendAtts = append(sendAtts, SendAttachment{
				Filename: a.Filename,
				Content:  data,
			})
		}

		client := NewMailClient(acc)
		err = client.SendEmail(ctx, args.To, args.Subject, args.Body, args.IsHTML, sendAtts)
		if err != nil {
			return nil, fmt.Errorf("sending email: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "Email sent successfully"}},
		}, nil
	})

	s.AddTool(&mcp.Tool{
		Name:        "reply_to_email",
		Description: "Reply to an existing email",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"properties":{
				"uid":{"type":"string","description":"Message UID to reply to"},
				"folder":{"type":"string","description":"Folder name"},
				"body":{"type":"string"},
				"is_html":{"type":"boolean"},
				"account_id":{"type":"string"}
			},
			"required":["uid","folder","body"]
		}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			UID       string `json:"uid"`
			Folder    string `json:"folder"`
			Body      string `json:"body"`
			IsHTML    bool   `json:"is_html"`
			AccountID string `json:"account_id"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return nil, fmt.Errorf("parsing arguments: %w", err)
		}

		uid, err := strconv.ParseUint(args.UID, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid UID %q: %w", args.UID, err)
		}

		acc, err := cfg.GetAccount(args.AccountID)
		if err != nil {
			return nil, fmt.Errorf("getting account: %w", err)
		}

		client := NewMailClient(acc)
		err = client.ReplyToEmail(ctx, args.Folder, uint32(uid), args.Body, args.IsHTML)
		if err != nil {
			return nil, fmt.Errorf("replying to email: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "Reply sent successfully"}},
		}, nil
	})

	// Management Tools
	s.AddTool(&mcp.Tool{
		Name:        "mark_as_read",
		Description: "Mark a message as read",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"uid":{"type":"string","description":"Message UID"},"folder":{"type":"string"},"account_id":{"type":"string"}},"required":["uid","folder"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			UID       string `json:"uid"`
			Folder    string `json:"folder"`
			AccountID string `json:"account_id"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return nil, fmt.Errorf("parsing arguments: %w", err)
		}

		uid, err := strconv.ParseUint(args.UID, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid UID %q: %w", args.UID, err)
		}

		acc, err := cfg.GetAccount(args.AccountID)
		if err != nil {
			return nil, fmt.Errorf("getting account: %w", err)
		}

		client := NewMailClient(acc)
		if err := client.MarkAsRead(ctx, args.Folder, uint32(uid)); err != nil {
			return nil, fmt.Errorf("marking as read: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "Message marked as read"}},
		}, nil
	})

	s.AddTool(&mcp.Tool{
		Name:        "delete_message",
		Description: "Permanently delete a message",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"uid":{"type":"string","description":"Message UID"},"folder":{"type":"string"},"account_id":{"type":"string"}},"required":["uid","folder"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			UID       string `json:"uid"`
			Folder    string `json:"folder"`
			AccountID string `json:"account_id"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return nil, fmt.Errorf("parsing arguments: %w", err)
		}

		uid, err := strconv.ParseUint(args.UID, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid UID %q: %w", args.UID, err)
		}

		acc, err := cfg.GetAccount(args.AccountID)
		if err != nil {
			return nil, fmt.Errorf("getting account: %w", err)
		}

		client := NewMailClient(acc)
		if err := client.DeleteMessage(ctx, args.Folder, uint32(uid)); err != nil {
			return nil, fmt.Errorf("deleting message: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "Message deleted"}},
		}, nil
	})
}
