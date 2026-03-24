package telegram

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/damonto/sigmo/internal/pkg/config"
	notifyevent "github.com/damonto/sigmo/internal/pkg/notify/event"
)

const defaultEndpoint = "https://api.telegram.org"

type Sender struct {
	client         *http.Client
	sendMessageURL string
	recipients     []int64
}

func New(cfg *config.Channel) (*Sender, error) {
	if strings.TrimSpace(cfg.BotToken) == "" {
		return nil, errors.New("telegram bot token is required")
	}
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	baseURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parsing telegram endpoint: %w", err)
	}
	recipients, err := cfg.Recipients.Int64s()
	if err != nil {
		return nil, fmt.Errorf("parsing telegram recipients: %w", err)
	}
	if len(recipients) == 0 {
		return nil, errors.New("telegram recipients are required")
	}

	sendMessageURL := *baseURL
	sendMessageURL.Path = path.Join(sendMessageURL.Path, "bot"+cfg.BotToken, "sendMessage")
	return &Sender{
		client:         &http.Client{Timeout: 10 * time.Second},
		sendMessageURL: sendMessageURL.String(),
		recipients:     recipients,
	}, nil
}

func (s *Sender) Send(ctx context.Context, ev notifyevent.Event) error {
	if len(s.recipients) == 0 {
		return errors.New("telegram recipients are required")
	}
	content, err := render(ev)
	if err != nil {
		return err
	}

	var combined error
	for _, recipient := range s.recipients {
		if err := s.sendOne(ctx, recipient, content); err != nil {
			combined = errors.Join(combined, err)
		}
	}
	return combined
}
