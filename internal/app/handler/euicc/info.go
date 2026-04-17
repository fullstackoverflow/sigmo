package euicc

import (
	"errors"
	"log/slog"

	"github.com/damonto/sigmo/internal/pkg/config"
	"github.com/damonto/sigmo/internal/pkg/lpa"
	mmodem "github.com/damonto/sigmo/internal/pkg/modem"
)

type euicc struct {
	cfg *config.Config
}

func newEUICC(cfg *config.Config) *euicc {
	return &euicc{
		cfg: cfg,
	}
}

func (e *euicc) Get(modem *mmodem.Modem) (*EuiccResponse, error) {
	client, err := lpa.New(modem, e.cfg)
	if err != nil {
		if errors.Is(err, lpa.ErrNoSupportedAID) {
			return nil, err
		}
		slog.Error("failed to create LPA client", "modem", modem.EquipmentIdentifier, "error", err)
		return nil, err
	}
	defer func() {
		if cerr := client.Close(); cerr != nil {
			slog.Warn("failed to close LPA client", "error", cerr)
		}
	}()

	info, err := client.Info()
	if err != nil {
		slog.Error("failed to fetch eUICC info", "modem", modem.EquipmentIdentifier, "error", err)
		return nil, err
	}
	return &EuiccResponse{
		EID:          info.EID,
		FreeSpace:    info.FreeSpace,
		SASUP:        info.SASUP,
		Certificates: info.Certificates,
	}, nil
}
