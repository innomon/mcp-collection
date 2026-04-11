# mail-mcp

A provider-agnostic MCP (Model Context Protocol) server for interacting with email accounts via IMAP and SMTP.

## Features

- **Pure Go**: No CGO dependencies.
- **IMAP (v2)**: Robust email fetching and folder management.
- **SMTP**: Clean API for sending emails with HTML and attachments.
- **Secure**: Supports OAuth2 and App Passwords for modern providers like Gmail and Zoho.

## Project Structure

- `SPEC.md`: Detailed architectural and tool specification.
- `conductor/`: Project planning and track management.
- `config.yaml.example`: Example configuration.

## Getting Started

1. Copy `config.yaml.example` to `config.yaml`.
2. Configure your IMAP and SMTP settings.
3. Build and run the server:
   ```bash
   go build .
   ./mail-mcp --config config.yaml
   ```

## Development

Follow the [Implementation Plan](./conductor/tracks/mail_server_core_20260411/plan.md) for detailed progress.
