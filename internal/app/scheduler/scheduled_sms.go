package scheduler

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/damonto/sigmo/internal/pkg/config"
	"github.com/damonto/sigmo/internal/pkg/modem"
)

const scheduledSMSPollInterval = 30 * time.Second

type ScheduledSMS struct {
	cfg      *config.Config
	manager  *modem.Manager
	interval time.Duration
}

func NewScheduledSMS(cfg *config.Config, manager *modem.Manager) *ScheduledSMS {
	return &ScheduledSMS{
		cfg:      cfg,
		manager:  manager,
		interval: scheduledSMSPollInterval,
	}
}

func (s *ScheduledSMS) Enabled() bool {
	return len(s.cfg.ScheduledSMSJobs()) > 0
}

func (s *ScheduledSMS) Run(ctx context.Context) error {
	if !s.Enabled() {
		<-ctx.Done()
		return nil
	}
	s.runOnce(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s *ScheduledSMS) runOnce(ctx context.Context) {
	_ = ctx
	jobs := s.cfg.ScheduledSMSJobs()
	if len(jobs) == 0 {
		return
	}

	modems, err := s.manager.Modems()
	if err != nil {
		slog.Error("failed to list modems for scheduled SMS", "error", err)
		return
	}
	modemsByID := make(map[string]*modem.Modem, len(modems))
	for _, m := range modems {
		modemsByID[m.EquipmentIdentifier] = m
	}

	now := time.Now().UTC()
	for _, job := range jobs {
		if !job.Enabled {
			continue
		}
		if err := validateJob(job); err != nil {
			slog.Warn("skipping invalid scheduled SMS job", "job", job.Name, "error", err)
			continue
		}
		if !job.NextSendAt.IsZero() && job.NextSendAt.After(now) {
			continue
		}

		targetModem, ok := modemsByID[job.ModemID]
		if !ok {
			slog.Warn("scheduled SMS modem not found", "job", job.Name, "modem_id", job.ModemID)
			continue
		}
		if _, err := targetModem.Messaging().Send(job.To, job.Text); err != nil {
			slog.Error("failed to send scheduled SMS", "job", job.Name, "modem", job.ModemID, "to", job.To, "error", err)
			continue
		}

		nextSendAt := now.AddDate(0, job.IntervalMonths, job.IntervalDays)
		if err := s.cfg.MarkScheduledSMSSent(job.Name, now, nextSendAt); err != nil {
			slog.Error("failed to persist scheduled SMS state", "job", job.Name, "error", err)
			continue
		}
		slog.Info("scheduled SMS sent", "job", job.Name, "modem", job.ModemID, "to", job.To, "next_send_at", nextSendAt)
	}
}

func validateJob(job config.ScheduledSMS) error {
	if strings.TrimSpace(job.Name) == "" {
		return errInvalidJob("name is required")
	}
	if strings.TrimSpace(job.ModemID) == "" {
		return errInvalidJob("modem_id is required")
	}
	if strings.TrimSpace(job.To) == "" {
		return errInvalidJob("to is required")
	}
	if strings.TrimSpace(job.Text) == "" {
		return errInvalidJob("text is required")
	}
	if job.IntervalMonths <= 0 && job.IntervalDays <= 0 {
		return errInvalidJob("interval_months or interval_days must be greater than 0")
	}
	return nil
}

type errInvalidJob string

func (e errInvalidJob) Error() string {
	return string(e)
}
