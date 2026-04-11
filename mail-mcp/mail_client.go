package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/mail"
	_ "github.com/emersion/go-message/charset"
	gomail "github.com/wneessen/go-mail"
)

type MailClient struct {
	Account *AccountConfig
}

func NewMailClient(acc *AccountConfig) *MailClient {
	return &MailClient{Account: acc}
}

// ConnectIMAP establishes a connection to the IMAP server and logs in.
func (c *MailClient) ConnectIMAP(ctx context.Context) (*imapclient.Client, error) {
	addr := fmt.Sprintf("%s:%d", c.Account.IMAP.Host, c.Account.IMAP.Port)
	var client *imapclient.Client
	var err error

	if c.Account.IMAP.TLS {
		client, err = imapclient.DialTLS(addr, nil)
	} else {
		// Use DialInsecure for plain-text connections in v2
		client, err = imapclient.DialInsecure(addr, nil)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to dial IMAP: %w", err)
	}

	// For now we support app_password (PLAIN auth)
	if c.Account.Auth.Type == "app_password" {
		if err := client.Login(c.Account.Auth.User, c.Account.Auth.Password).Wait(); err != nil {
			client.Close()
			return nil, fmt.Errorf("IMAP login failed: %w", err)
		}
	} else {
		client.Close()
		return nil, fmt.Errorf("unsupported auth type: %s", c.Account.Auth.Type)
	}

	return client, nil
}

type Folder struct {
	Name       string   `json:"name"`
	Attributes []string `json:"attributes"`
}

func (c *MailClient) ListFolders(ctx context.Context) ([]Folder, error) {
	client, err := c.ConnectIMAP(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Logout()

	mailboxes, err := client.List("", "*", nil).Collect()
	if err != nil {
		return nil, fmt.Errorf("failed to list mailboxes: %w", err)
	}

	var folders []Folder
	for _, m := range mailboxes {
		var attrs []string
		for _, a := range m.Attrs {
			attrs = append(attrs, string(a))
		}
		folders = append(folders, Folder{
			Name:       m.Mailbox,
			Attributes: attrs,
		})
	}
	return folders, nil
}

type MessageHeader struct {
	UID       uint32 `json:"uid"`
	SeqNum    uint32 `json:"seq_num"`
	Subject   string `json:"subject"`
	From      string `json:"from"`
	Date      string `json:"date"`
	MessageID string `json:"message_id"`
}

func (c *MailClient) ListMessages(ctx context.Context, folder string, limit int) ([]MessageHeader, error) {
	client, err := c.ConnectIMAP(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Logout()

	mbox, err := client.Select(folder, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("failed to select folder %s: %w", folder, err)
	}

	if mbox.NumMessages == 0 {
		return nil, nil
	}

	start := uint32(1)
	if mbox.NumMessages > uint32(limit) {
		start = mbox.NumMessages - uint32(limit) + 1
	}

	var seqSet imap.SeqSet
	seqSet.AddRange(start, mbox.NumMessages)

	fetchOptions := &imap.FetchOptions{
		Envelope: true,
		UID:      true,
	}

	messages, err := client.Fetch(seqSet, fetchOptions).Collect()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch messages: %w", err)
	}

	var headers []MessageHeader
	// Reverse to show newest first
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		h := MessageHeader{
			UID:    uint32(msg.UID), // Cast imap.UID to uint32
			SeqNum: msg.SeqNum,
		}
		if msg.Envelope != nil {
			h.Subject = msg.Envelope.Subject
			h.Date = msg.Envelope.Date.String()
			h.MessageID = msg.Envelope.MessageID
			if len(msg.Envelope.From) > 0 {
				f := msg.Envelope.From[0]
				h.From = fmt.Sprintf("%s <%s@%s>", f.Name, f.Mailbox, f.Host)
			}
		}
		headers = append(headers, h)
	}

	return headers, nil
}

type Attachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

type MessageDetail struct {
	MessageHeader
	Body        string       `json:"body"`
	HTMLBody    string       `json:"html_body"`
	Attachments []Attachment `json:"attachments"`
}

type SendAttachment struct {
	Filename string `json:"filename"`
	Content  []byte `json:"content"`
}

func (c *MailClient) GetMessage(ctx context.Context, folder string, uid uint32) (*MessageDetail, error) {
	client, err := c.ConnectIMAP(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Logout()

	_, err = client.Select(folder, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("failed to select folder %s: %w", folder, err)
	}

	// Use UIDSetNum to create a set containing the single UID
	uidSet := imap.UIDSetNum(imap.UID(uid))

	// Fetch the full message body (RFC822) to parse it
	bodySection := &imap.FetchItemBodySection{}
	fetchOptions := &imap.FetchOptions{
		Envelope:    true,
		UID:         true,
		BodySection: []*imap.FetchItemBodySection{bodySection},
	}

	// In v2, Fetch handles both sequence numbers and UIDs based on the set type
	messages, err := client.Fetch(uidSet, fetchOptions).Collect()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch message: %w", err)
	}

	if len(messages) == 0 {
		return nil, fmt.Errorf("message not found")
	}

	msg := messages[0]
	detail := &MessageDetail{
		MessageHeader: MessageHeader{
			UID:       uint32(msg.UID),
			SeqNum:    msg.SeqNum,
			MessageID: msg.Envelope.MessageID,
		},
	}
	if msg.Envelope != nil {
		detail.Subject = msg.Envelope.Subject
		detail.Date = msg.Envelope.Date.String()
		if len(msg.Envelope.From) > 0 {
			f := msg.Envelope.From[0]
			detail.From = fmt.Sprintf("%s <%s@%s>", f.Name, f.Mailbox, f.Host)
		}
	}

	bodyData := msg.FindBodySection(bodySection)
	mr, err := mail.CreateReader(bytes.NewReader(bodyData))
	if err != nil {
		// Fallback to raw body if parsing fails
		detail.Body = string(bodyData)
		return detail, nil
	}
	defer mr.Close()

	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			break
		}

		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			ct, _, _ := h.ContentType()
			b, _ := io.ReadAll(p.Body)
			if ct == "text/html" {
				detail.HTMLBody = string(b)
			} else {
				detail.Body = string(b)
			}
		case *mail.AttachmentHeader:
			filename, _ := h.Filename()
			ct, _, _ := h.ContentType()
			// We don't read the whole attachment into memory here for the detail view,
			// just the metadata.
			detail.Attachments = append(detail.Attachments, Attachment{
				Filename:    filename,
				ContentType: ct,
			})
		}
	}

	return detail, nil
}

// ReplyToEmail sends a reply to an existing message.
func (c *MailClient) ReplyToEmail(ctx context.Context, folder string, uid uint32, body string, isHTML bool) error {
	original, err := c.GetMessage(ctx, folder, uid)
	if err != nil {
		return fmt.Errorf("getting original message: %w", err)
	}

	m := gomail.NewMsg()
	if err := m.From(c.Account.Email); err != nil {
		return fmt.Errorf("failed to set from: %w", err)
	}
	if err := m.To(original.From); err != nil { // Reply to sender
		return fmt.Errorf("failed to set to: %w", err)
	}

	subject := original.Subject
	if !strings.HasPrefix(strings.ToLower(subject), "re:") {
		subject = "Re: " + subject
	}
	m.Subject(subject)

	if isHTML {
		m.SetBodyString(gomail.TypeTextHTML, body)
	} else {
		m.SetBodyString(gomail.TypeTextPlain, body)
	}

	// Set threading headers
	if original.MessageID != "" {
		m.SetGenHeader("In-Reply-To", original.MessageID)
		m.SetGenHeader("References", original.MessageID)
	}

	opts := []gomail.Option{
		gomail.WithPort(c.Account.SMTP.Port),
		gomail.WithUsername(c.Account.Auth.User),
		gomail.WithPassword(c.Account.Auth.Password),
		gomail.WithSMTPAuth(gomail.SMTPAuthPlain),
	}

	if c.Account.SMTP.StartTLS {
		opts = append(opts, gomail.WithTLSPolicy(gomail.TLSMandatory))
	} else {
		opts = append(opts, gomail.WithTLSPolicy(gomail.NoTLS))
	}

	client, err := gomail.NewClient(c.Account.SMTP.Host, opts...)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}

	if err := client.DialAndSendWithContext(ctx, m); err != nil {
		return fmt.Errorf("failed to send reply: %w", err)
	}

	return nil
}

func (c *MailClient) MarkAsRead(ctx context.Context, folder string, uid uint32) error {
	client, err := c.ConnectIMAP(ctx)
	if err != nil {
		return err
	}
	defer client.Logout()

	_, err = client.Select(folder, nil).Wait()
	if err != nil {
		return fmt.Errorf("failed to select folder %s: %w", folder, err)
	}

	uidSet := imap.UIDSetNum(imap.UID(uid))
	storeFlags := &imap.StoreFlags{
		Op:    imap.StoreFlagsAdd,
		Flags: []imap.Flag{imap.FlagSeen},
	}

	if err := client.Store(uidSet, storeFlags, nil).Close(); err != nil {
		return fmt.Errorf("failed to mark as read: %w", err)
	}

	return nil
}

func (c *MailClient) DeleteMessage(ctx context.Context, folder string, uid uint32) error {
	client, err := c.ConnectIMAP(ctx)
	if err != nil {
		return err
	}
	defer client.Logout()

	_, err = client.Select(folder, nil).Wait()
	if err != nil {
		return fmt.Errorf("failed to select folder %s: %w", folder, err)
	}

	uidSet := imap.UIDSetNum(imap.UID(uid))
	storeFlags := &imap.StoreFlags{
		Op:    imap.StoreFlagsAdd,
		Flags: []imap.Flag{imap.FlagDeleted},
	}

	if err := client.Store(uidSet, storeFlags, nil).Close(); err != nil {
		return fmt.Errorf("failed to mark as deleted: %w", err)
	}

	if err := client.Expunge().Close(); err != nil {
		return fmt.Errorf("failed to expunge: %w", err)
	}

	return nil
}

// SendEmail sends an email using the SMTP configuration of the account.
func (c *MailClient) SendEmail(ctx context.Context, to []string, subject, body string, isHTML bool, attachments []SendAttachment) error {
	m := gomail.NewMsg()
	if err := m.From(c.Account.Email); err != nil {
		return fmt.Errorf("failed to set from: %w", err)
	}
	if err := m.To(to...); err != nil {
		return fmt.Errorf("failed to set to: %w", err)
	}
	m.Subject(subject)
	if isHTML {
		m.SetBodyString(gomail.TypeTextHTML, body)
	} else {
		m.SetBodyString(gomail.TypeTextPlain, body)
	}

	for _, att := range attachments {
		m.AttachReader(att.Filename, bytes.NewReader(att.Content))
	}

	opts := []gomail.Option{
		gomail.WithPort(c.Account.SMTP.Port),
		gomail.WithUsername(c.Account.Auth.User),
		gomail.WithPassword(c.Account.Auth.Password),
		gomail.WithSMTPAuth(gomail.SMTPAuthPlain),
	}

	if c.Account.SMTP.StartTLS {
		opts = append(opts, gomail.WithTLSPolicy(gomail.TLSMandatory))
	} else {
		opts = append(opts, gomail.WithTLSPolicy(gomail.NoTLS))
	}

	client, err := gomail.NewClient(c.Account.SMTP.Host, opts...)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}

	if err := client.DialAndSendWithContext(ctx, m); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}
