package main

import (
	"context"
	"fmt"

	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/wneessen/go-mail"
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
		client, err = imapclient.Dial(addr, nil)
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

// SendEmail sends an email using the SMTP configuration of the account.
func (c *MailClient) SendEmail(ctx context.Context, to []string, subject, body string, isHTML bool) error {
	m := mail.NewMsg()
	if err := m.From(c.Account.Email); err != nil {
		return fmt.Errorf("failed to set from: %w", err)
	}
	if err := m.To(to...); err != nil {
		return fmt.Errorf("failed to set to: %w", err)
	}
	m.Subject(subject)
	if isHTML {
		m.SetBodyString(mail.TypeTextHTML, body)
	} else {
		m.SetBodyString(mail.TypeTextPlain, body)
	}

	opts := []mail.Option{
		mail.WithPort(c.Account.SMTP.Port),
		mail.WithSMTPAuth(mail.SMTPAuthPlain, c.Account.Auth.User, c.Account.Auth.Password),
	}

	if c.Account.SMTP.StartTLS {
		opts = append(opts, mail.WithTLSPolicy(mail.TLSMandatory))
	} else {
		opts = append(opts, mail.WithTLSPolicy(mail.TLSNoTLS))
	}

	client, err := mail.NewClient(c.Account.SMTP.Host, opts...)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}

	if err := client.DialAndSendWithContext(ctx, m); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}
