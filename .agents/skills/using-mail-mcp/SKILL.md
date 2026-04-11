---
name: using-mail-mcp
description: Instructions for using the Mail MCP server to read, search, send, and manage emails via IMAP and SMTP. Use when the user wants to check their inbox, search for specific messages, send or reply to emails, or manage mail folders.
---

# Using Mail MCP

The `mail-mcp` server provides a standard interface for interacting with email accounts. It supports common operations like listing folders, fetching messages, and sending emails.

## Core Workflows

### 1. Checking for New Mail
To check for new mail, first list the available folders to find the correct inbox name, then list recent messages.

1.  **List Folders**: Call `list_folders` to see all available mail folders (e.g., `INBOX`, `Sent`, `Junk`).
2.  **List Messages**: Call `list_messages` with the desired `folder` (usually `INBOX`) and an optional `limit`.
3.  **Read Content**: For interesting messages, call `get_message` with the `uid` and `folder`.

### 2. Sending Emails
You can send new emails or reply to existing ones.

*   **New Email**: Use `send_email`. You must provide `to` (as an array), `subject`, and `body`. You can optionally set `is_html: true` for HTML content.
*   **Reply**: Use `reply_to_email`. This requires the `uid` and `folder` of the original message. It automatically handles `In-Reply-To` and `References` headers.

### 3. Managing Messages
*   **Mark as Read**: Use `mark_as_read` with `uid` and `folder`.
*   **Delete**: Use `delete_message` with `uid` and `folder`. Note that this is a permanent deletion (depending on the server configuration).

## Tool Reference

| Tool | Key Arguments | Usage Note |
| :--- | :--- | :--- |
| `list_folders` | `account_id` (opt) | Always start here if unsure of folder names. |
| `list_messages` | `folder`, `limit` (opt), `account_id` (opt) | Returns UID, From, Subject, and Date. |
| `get_message` | `uid`, `folder`, `account_id` (opt) | Fetches the full body and metadata. |
| `send_email` | `to`, `subject`, `body`, `is_html` (opt), `attachments` (opt) | `attachments` are `{filename, content}` (base64). |
| `reply_to_email` | `uid`, `folder`, `body`, `is_html` (opt) | Simplifies threading. |
| `mark_as_read` | `uid`, `folder` | Updates the `\Seen` flag. |
| `delete_message` | `uid`, `folder` | Permanently deletes the message. |

## Important Notes
- **UIDs**: UIDs are specific to a folder. Always provide the correct `folder` name when using a `uid`.
- **Multiple Accounts**: If the server is configured with multiple accounts, use the `account_id` to specify which one to use. If omitted, the first account in the configuration is used.
- **Attachments**: When sending attachments, the `content` must be a base64-encoded string.
