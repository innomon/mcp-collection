# Mail MCP Server — Specification

## 1. Overview

The `mail-mcp` server provides a standard interface for AI agents to interact with email accounts via **IMAP** (for reading) and **SMTP** (for sending). It is designed to be provider-agnostic, supporting Gmail, Zoho, and private mail servers while maintaining a "pure Go" implementation (no CGO).

### Architecture Summary

```
┌──────────────────────────────────────────────────────────────┐
│                      mail-mcp server                         │
│                                                              │
│  ┌────────────┐  ┌────────────────────────────────────────┐  │
│  │ MCP Tools  │  │ Configuration (config.yaml)            │  │
│  │ (JSON-RPC) │  │ - IMAP/SMTP Settings                   │  │
│  └─────┬──────┘  │ - Auth (OAuth2 / App Passwords)        │  │
│        │         └────────────────────────────────────────┘  │
│        │                                                     │
│  ┌─────▼───────────────▼──────┐                              │
│  │      Capability Handlers   │                              │
│  │  ┌─────────┐ ┌──────────┐  │                              │
│  │  │ Fetch   │ │ Send     │  │                              │
│  │  ├─────────┤ ├──────────┤  │                              │
│  │  │ Search  │ │ Delete   │  │                              │
│  │  ├─────────┤ ├──────────┤  │                              │
│  │  │ Folders │ │ Mark Read│  │                              │
│  │  └─────────┘ └──────────┘  │                              │
│  └─────────────┬──────────────┘                              │
│                │                                             │
│  ┌─────────────▼──────────────┐   ┌────────────────────────┐ │
│  │  IMAP Client (v2)          │   │  SMTP Client           │ │
│  │  (emersion/go-imap)        │   │  (wneessen/go-mail)    │ │
│  └────────────────────────────┘   └────────────────────────┘ │
└──────────────────────────────────────────────────────────────┘
         │                                  │
         ▼                                  ▼
   IMAP Server (Port 993)            SMTP Server (Port 587/465)
```

---

## 2. Technical Stack

* **Language:** Go (Golang) 1.22+
* **MCP Framework:** `github.com/modelcontextprotocol/go-sdk`
* **IMAP (Receiving):** `github.com/emersion/go-imap/v2`
* **MIME/Message Parsing:** `github.com/emersion/go-message`
* **SMTP (Sending):** `github.com/wneessen/go-mail`
* **Auth:** `github.com/emersion/go-sasl` (for OAuth2/App Passwords)
* **Configuration:** YAML (as per project pattern)

---

## 3. Tool Specifications

### A. Reading & Searching

| Tool | Description |
|------|-------------|
| `list_folders` | List all mail folders (INBOX, Sent, etc.) |
| `list_messages` | List recent messages in a folder (metadata only) |
| `search_messages` | Search messages by criteria (from, to, subject, body) |
| `get_message` | Fetch full message content including body and attachments |

### B. Composition & Sending

| Tool | Description |
|------|-------------|
| `send_email` | Send a new email with support for HTML and attachments |
| `reply_to_email` | Reply to an existing thread (handles In-Reply-To/References) |

### C. Management

| Tool | Description |
|------|-------------|
| `mark_as_read` | Mark message(s) as read |
| `move_message` | Move message to another folder (e.g., Trash) |
| `delete_message` | Permanently delete or move to Trash |

---

## 4. Configuration Schema (`config.yaml`)

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
      type: "app_password" # or "oauth2"
      user: "user@example.com"
      password: "secret-app-password"
```

---

## 5. Security & Privacy

* **No Credentials in Logs:** Ensure passwords and OAuth tokens are never logged.
* **TLS Mandatory:** Enforce TLS for all connections.
* **Attachment Safety:** Sanitize attachment filenames and mime types before exposing to the agent.
