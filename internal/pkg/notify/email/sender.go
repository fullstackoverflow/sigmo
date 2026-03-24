package email

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/damonto/sigmo/internal/pkg/config"
	notifyevent "github.com/damonto/sigmo/internal/pkg/notify/event"
	"github.com/wneessen/go-mail"
)

type Sender struct {
	client     *mail.Client
	from       string
	recipients []string
}

func New(cfg *config.Channel) (*Sender, error) {
	host := strings.TrimSpace(cfg.SMTPHost)
	if host == "" {
		return nil, errors.New("email smtp_host is required")
	}
	if cfg.SMTPPort <= 0 {
		return nil, errors.New("email smtp_port is required")
	}
	from := strings.TrimSpace(cfg.From)
	if from == "" {
		return nil, errors.New("email from is required")
	}
	recipients := cfg.Recipients.Strings()
	if len(recipients) == 0 {
		return nil, errors.New("email recipients are required")
	}

	tlsPolicy, err := parseTLSPolicy(cfg.TLSPolicy)
	if err != nil {
		return nil, err
	}

	options := []mail.Option{mail.WithPort(cfg.SMTPPort)}
	if cfg.SSL {
		options = append(options, mail.WithSSLPort(true))
	}

	username := strings.TrimSpace(cfg.SMTPUsername)
	password := strings.TrimSpace(cfg.SMTPPassword)
	if username != "" || password != "" {
		if username == "" || password == "" {
			return nil, errors.New("email smtp_username and smtp_password must be set together")
		}
		options = append(options,
			mail.WithSMTPAuth(mail.SMTPAuthAutoDiscover),
			mail.WithUsername(username),
			mail.WithPassword(password),
		)
	}

	client, err := mail.NewClient(host, options...)
	if err != nil {
		return nil, fmt.Errorf("creating email client: %w", err)
	}
	client.SetTLSPolicy(tlsPolicy)

	return &Sender{
		client:     client,
		from:       from,
		recipients: recipients,
	}, nil
}

func (s *Sender) Send(ctx context.Context, ev notifyevent.Event) error {
	if len(s.recipients) == 0 {
		return errors.New("email recipients are required")
	}
	content, err := render(ev)
	if err != nil {
		return err
	}

	msg := mail.NewMsg()
	if err := msg.From(s.from); err != nil {
		return fmt.Errorf("setting email from: %w", err)
	}
	if err := msg.To(s.recipients...); err != nil {
		return fmt.Errorf("setting email recipients: %w", err)
	}
	msg.Subject(content.Subject)
	msg.SetBodyString(mail.TypeTextPlain, content.TextBody)
	if strings.TrimSpace(content.HTMLBody) != "" {
		msg.AddAlternativeString(mail.TypeTextHTML, content.HTMLBody)
	}

	if err := s.client.DialAndSendWithContext(ctx, msg); err != nil {
		return fmt.Errorf("sending email: %w", err)
	}
	return nil
}
