# Core Mail Server - Implementation Plan

## Phase 1: Foundation

- [x] Initialize `go.mod` in `mail-mcp/`
- [x] Add dependencies:
  - `github.com/modelcontextprotocol/go-sdk`
  - `github.com/emersion/go-imap/v2`
  - `github.com/emersion/go-message`
  - `github.com/wneessen/go-mail`
  - `github.com/emersion/go-sasl`
- [x] Implement `config.go` to parse `config.yaml`
- [x] Implement basic `main.go` with MCP server initialization

## Phase 2: Authentication & Connectivity

- [x] Create `mail_client.go` to handle IMAP and SMTP connections (logic for `auth.go` merged here for simplicity)
- [x] Implement `imap_client.go` logic (integrated into `mail_client.go`) to handle IMAP connection and authentication
- [x] Implement `smtp_client.go` logic (integrated into `mail_client.go`) for SMTP connection handling

## Phase 3: Reading Tools (IMAP)

- [x] Implement `list_folders` tool
- [x] Implement `list_messages` tool (fetches ENVELOPE)
- [x] Implement `get_message` tool
  - [x] Use `go-message` to parse MIME parts
  - [x] Extract plain text and HTML bodies
  - [x] Handle attachment metadata

## Phase 4: Composition Tools (SMTP)

- [x] Implement `send_email` tool
  - [x] Support plain text and HTML
  - [x] Basic attachment support
- [x] Implement `reply_to_email` tool (handles In-Reply-To/References)


## Phase 5: Management Tools

- [x] Implement `mark_as_read` tool
- [x] Implement `delete_message` tool (moves to Trash if configured - currently does permanent delete with Expunge)

## Phase 6: Refinement & Testing

- [x] Comprehensive error handling for connection issues
- [x] Sanitize agent inputs for security
- [x] Unit tests for config parsing and message mapping
- [ ] End-to-end manual testing with a real email account (e.g., Gmail App Password)

## Checklist Verification
- [x] `go build ./mail-mcp/...` succeeds
- [x] `config.yaml.example` is present
- [x] `README.md` is updated with usage instructions
