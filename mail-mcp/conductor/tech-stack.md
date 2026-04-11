# Tech Stack - mail-mcp

## Core
- **Language**: Go 1.22+
- **SDK**: github.com/modelcontextprotocol/go-sdk/mcp

## Email Protocols
- **IMAP**: github.com/emersion/go-imap/v2
- **SMTP**: github.com/wneessen/go-mail
- **MIME Parsing**: github.com/emersion/go-message
- **SASL (Auth)**: github.com/emersion/go-sasl

## Testing
- **Suite**: standard `testing` package
- **Mocks**: Handcrafted or interface-based mocks for IMAP/SMTP servers.
