package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		content        string
		wantErr        string
		wantOTPDefault bool
	}{
		{
			name: "defaults otp_required when omitted",
			content: `
[app]
environment = "development"
listen_address = "127.0.0.1:9527"

[channels.telegram]
bot_token = "token"
recipients = [123456]
`,
			wantOTPDefault: true,
		},
		{
			name: "fails on unknown subject field",
			content: `
[app]
environment = "development"
listen_address = "127.0.0.1:9527"

[channels.bark]
endpoint = "https://api.day.app"
recipients = ["device-key"]
subject = "deprecated"
`,
			wantErr: "unknown config fields: channels.bark.subject",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(path, []byte(strings.TrimSpace(tt.content)), 0o644); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}

			got, err := Load(path)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Load() error = nil, want %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Load() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if got.App.OTPRequired != tt.wantOTPDefault {
				t.Fatalf("OTPRequired = %v, want %v", got.App.OTPRequired, tt.wantOTPDefault)
			}
		})
	}
}
