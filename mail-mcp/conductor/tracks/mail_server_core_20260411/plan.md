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

- [ ] Implement `list_folders` tool
- [ ] Implement `list_messages` tool (fetches ENVELOPE)
- [ ] Implement `get_message` tool
  - [ ] Use `go-message` to parse MIME parts
  - [ ] Extract plain text and HTML bodies
  - [ ] Handle attachment metadata

## Phase 4: Composition Tools (SMTP)

- [ ] Implement `send_email` tool
  - [ ] Support plain text and HTML
  - [ ] Basic attachment support

## Phase 5: Management Tools

- [ ] Implement `mark_as_read` tool
- [ ] Implement `delete_message` tool (moves to Trash if configured)

## Phase 6: Refinement & Testing

- [ ] Comprehensive error handling for connection issues
- [ ] Sanitize agent inputs for security
- [ ] Unit tests for config parsing and message mapping
- [ ] End-to-end manual testing with a real email account (e.g., Gmail App Password)

## Checklist Verification
- [ ] `go build ./mail-mcp/...` succeeds
- [ ] `config.yaml.example` is present
- [ ] `README.md` is updated with usage instructions
