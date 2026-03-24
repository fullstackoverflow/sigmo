package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

type message struct {
	ChatID    int64  `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

func (s *Sender) sendOne(ctx context.Context, recipient int64, content content) error {
	body, err := json.Marshal(message{
		ChatID:    recipient,
		Text:      content.Text,
		ParseMode: content.ParseMode,
	})
	if err != nil {
		return fmt.Errorf("encoding telegram message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.sendMessageURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		slog.Error("failed to send telegram message", "recipient", recipient, "error", err)
		return fmt.Errorf("sending telegram message: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("telegram response status %s: %s", resp.Status, strings.TrimSpace(string(payload)))
	}
	return nil
}
