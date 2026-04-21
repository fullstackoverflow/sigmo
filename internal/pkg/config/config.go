package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// Config represents the application configuration
type Config struct {
	App          App                `toml:"app"`
	Channels     map[string]Channel `toml:"channels"`
	Modems       map[string]Modem   `toml:"modems"`
	ScheduledSMS []ScheduledSMS     `toml:"scheduled_sms"`
	Path         string             `toml:"-"`

	mu sync.RWMutex `toml:"-"`
}

type App struct {
	Environment   string   `toml:"environment"`
	ListenAddress string   `toml:"listen_address"`
	AuthProviders []string `toml:"auth_providers"`
	OTPRequired   bool     `toml:"otp_required"`
}

type Channel struct {
	Endpoint string `toml:"endpoint,omitempty"`

	// Telegram
	BotToken   string     `toml:"bot_token,omitempty"`
	Recipients Recipients `toml:"recipients,omitempty"`

	// HTTP
	Headers map[string]string `toml:"headers,omitempty"`

	// Email
	SMTPHost     string `toml:"smtp_host,omitempty"`
	SMTPPort     int    `toml:"smtp_port,omitempty"`
	SMTPUsername string `toml:"smtp_username,omitempty"`
	SMTPPassword string `toml:"smtp_password,omitempty"`
	From         string `toml:"from,omitempty"`
	TLSPolicy    string `toml:"tls_policy,omitempty"`
	SSL          bool   `toml:"ssl,omitempty"`

	// Gotify
	Priority int `toml:"priority,omitempty"`
}

type Modem struct {
	Alias      string `toml:"alias"`
	Compatible bool   `toml:"compatible"`
	MSS        int    `toml:"mss"`
}

type ScheduledSMS struct {
	Name           string    `toml:"name"`
	Enabled        bool      `toml:"enabled"`
	ModemID        string    `toml:"modem_id"`
	To             string    `toml:"to"`
	Text           string    `toml:"text"`
	IntervalMonths int       `toml:"interval_months,omitempty"`
	IntervalDays   int       `toml:"interval_days,omitempty"`
	NextSendAt     time.Time `toml:"next_send_at,omitempty"`
	LastSentAt     time.Time `toml:"last_sent_at,omitempty"`
}

// Load reads and parses the configuration from the given file path
func Load(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config: %w", err)
	}
	defer file.Close()

	var config Config
	if err := toml.NewDecoder(file).DisallowUnknownFields().EnableUnmarshalerInterface().Decode(&config); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	config.Path = path
	return &config, nil
}

func (c *Config) IsProduction() bool {
	return c.App.Environment == "production"
}

func (c *Config) FindModem(id string) Modem {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if modem, ok := c.Modems[id]; ok {
		return modem
	}
	return Modem{
		Compatible: false,
		MSS:        240,
	}
}

func (c *Config) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.saveLocked()
}

func (c *Config) ScheduledSMSJobs() []ScheduledSMS {
	c.mu.RLock()
	defer c.mu.RUnlock()

	jobs := make([]ScheduledSMS, len(c.ScheduledSMS))
	copy(jobs, c.ScheduledSMS)
	return jobs
}

func (c *Config) UpdateModem(id string, modem Modem) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Modems == nil {
		c.Modems = make(map[string]Modem)
	}
	c.Modems[id] = modem
	return c.saveLocked()
}

func (c *Config) MarkScheduledSMSSent(name string, lastSentAt, nextSendAt time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := range c.ScheduledSMS {
		if c.ScheduledSMS[i].Name != name {
			continue
		}
		c.ScheduledSMS[i].LastSentAt = lastSentAt
		c.ScheduledSMS[i].NextSendAt = nextSendAt
		return c.saveLocked()
	}
	return fmt.Errorf("scheduled sms job not found: %s", name)
}

func (c *Config) saveLocked() error {
	if c.Path == "" {
		return errors.New("config path is required")
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(c); err != nil {
		return fmt.Errorf("encoding config file: %w", err)
	}
	if err := os.WriteFile(c.Path, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}
	return nil
}
