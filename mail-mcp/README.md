# mail-mcp

A provider-agnostic MCP (Model Context Protocol) server for interacting with email accounts via IMAP and SMTP.

## Features

- **Pure Go**: No CGO dependencies.
- **IMAP (v2)**: Robust email fetching, mailbox listing, and message management.
- **SMTP**: Clean API for sending emails with HTML and attachments (metadata extraction).
- **Secure**: Supports App Passwords and TLS/STARTTLS.
- **Multi-account**: Support for multiple email accounts in configuration.

## Tools

- `list_folders`: List all mail folders.
- `list_messages`: List recent messages in a folder with metadata.
- `get_message`: Fetch full message content including plain text and HTML bodies.
- `send_email`: Send a new email with support for attachments.
- `reply_to_email`: Reply to an existing email (handles threading).
- `mark_as_read`: Mark a message as seen.
- `delete_message`: Permanently delete a message (Expunge).

## Installation

```bash
go build -o mail-mcp .
```

## Configuration

Create a `config.yaml` file (see `config.yaml.example`):

```yaml
server:
  name: "mail-mcp"
  version: "1.0.0"

accounts:
  - id: "primary"
    email: "user@example.com"
    imap:
      host: "imap.gmail.com"
      port: 993
      tls: true
    smtp:
      host: "smtp.gmail.com"
      port: 587
      starttls: true
    auth:
      type: "app_password"
      user: "user@example.com"
      password: "your-app-password"
```

## Usage

Run the server with the config file:

```bash
./mail-mcp config.yaml
```

The server uses the `stdio` transport by default, making it compatible with MCP-compliant AI agents.
