package email

import (
	"fmt"
	"strings"

	"github.com/wneessen/go-mail"
)

func parseTLSPolicy(raw string) (mail.TLSPolicy, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "", "mandatory":
		return mail.TLSMandatory, nil
	case "opportunistic":
		return mail.TLSOpportunistic, nil
	case "none", "notls", "no_tls":
		return mail.NoTLS, nil
	default:
		return mail.TLSMandatory, fmt.Errorf("unsupported email tls_policy: %q", raw)
	}
}
