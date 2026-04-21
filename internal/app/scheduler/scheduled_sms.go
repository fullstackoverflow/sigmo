package scheduler

import (
	"context"
	"fmt"
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
	jobs, err := s.enabledJobs()
	if err != nil {
		slog.Error("scheduled SMS configuration is invalid", "error", err)
		return false
	}
	return len(jobs) > 0
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
	jobs, err := s.enabledJobs()
	if err != nil {
		slog.Error("skipping scheduled SMS run due to invalid configuration", "error", err)
		return
	}
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
		if job.NextSendAt.IsZero() {
			nextSendAt := calculateNextSendAt(now, job)
			if err := s.cfg.SetScheduledSMSNextSendAt(job.Name, nextSendAt); err != nil {
				slog.Error("failed to initialize scheduled SMS next send time", "job", job.Name, "error", err)
				continue
			}
			slog.Info("initialized scheduled SMS next_send_at", "job", job.Name, "next_send_at", nextSendAt)
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

		nextSendAt := calculateNextSendAt(now, job)
		if err := s.cfg.MarkScheduledSMSSent(job.Name, now, nextSendAt); err != nil {
			slog.Error("failed to persist scheduled SMS state", "job", job.Name, "error", err)
			continue
		}
		slog.Info("scheduled SMS sent", "job", job.Name, "modem", job.ModemID, "to", job.To, "next_send_at", nextSendAt)
	}
}

func (s *ScheduledSMS) enabledJobs() ([]config.ScheduledSMS, error) {
	all := s.cfg.ScheduledSMSJobs()
	enabled := make([]config.ScheduledSMS, 0, len(all))
	names := make(map[string]struct{}, len(all))

	for _, job := range all {
		if !job.Enabled {
			continue
		}
		if err := validateJob(job); err != nil {
			return nil, fmt.Errorf("invalid scheduled SMS job %q: %w", strings.TrimSpace(job.Name), err)
		}
		name := strings.TrimSpace(job.Name)
		if _, exists := names[name]; exists {
			return nil, fmt.Errorf("duplicate scheduled SMS job name: %s", name)
		}
		names[name] = struct{}{}
		enabled = append(enabled, job)
	}
	return enabled, nil
}

func calculateNextSendAt(base time.Time, job config.ScheduledSMS) time.Time {
	return base.AddDate(0, job.IntervalMonths, job.IntervalDays).
		Add(time.Duration(job.IntervalMinutes) * time.Minute)
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
	if job.IntervalMonths <= 0 && job.IntervalDays <= 0 && job.IntervalMinutes <= 0 {
		return errInvalidJob("interval_months, interval_days, or interval_minutes must be greater than 0")
	}
	return nil
}

type errInvalidJob string

func (e errInvalidJob) Error() string {
	return string(e)
}
