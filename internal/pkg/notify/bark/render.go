package bark

import (
	"fmt"
	"strings"

	notifyevent "github.com/damonto/sigmo/internal/pkg/notify/event"
)

type content struct {
	Title string
	Body  string
	Icon  string
}

func render(ev notifyevent.Event) (content, error) {
	switch ev := ev.(type) {
	case notifyevent.OTPEvent:
		code := strings.TrimSpace(ev.Code)
		return content{
			Title: "Sigmo Login",
			Body:  fmt.Sprintf("Your verification code is %s", code),
		}, nil
	case notifyevent.SMSEvent:
		return content{
			Title: ev.Counterparty(),
			Body:  ev.DisplayText(),
		}, nil
	default:
		return content{}, fmt.Errorf("rendering bark content for %q: unsupported event", ev.Kind())
	}
}
