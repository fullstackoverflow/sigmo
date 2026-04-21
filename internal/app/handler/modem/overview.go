package modem

import (
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"

	"github.com/damonto/sigmo/internal/pkg/carrier"
	"github.com/damonto/sigmo/internal/pkg/config"
	"github.com/damonto/sigmo/internal/pkg/lpa"
	mmodem "github.com/damonto/sigmo/internal/pkg/modem"
)

type catalog struct {
	cfg     *config.Config
	manager *mmodem.Manager
	mu      sync.RWMutex
	esim    map[string]bool
}

func newCatalog(cfg *config.Config, manager *mmodem.Manager) *catalog {
	return &catalog{
		cfg:     cfg,
		manager: manager,
		esim:    make(map[string]bool),
	}
}

func (c *catalog) List() ([]*ModemResponse, error) {
	modems, err := c.manager.Modems()
	if err != nil {
		slog.Error("failed to list modems", "error", err)
		return nil, err
	}
	type result struct {
		resp *ModemResponse
		err  error
	}
	response := make([]*ModemResponse, 0, len(modems))
	results := make(chan result, len(modems))
	var wg sync.WaitGroup
	for _, device := range modems {
		device := device
		wg.Add(1)
		go func() {
			defer wg.Done()
			modemResp, err := c.buildResponse(device)
			results <- result{resp: modemResp, err: err}
		}()
	}
	wg.Wait()
	close(results)

	for item := range results {
		if item.err != nil {
			return nil, item.err
		}
		response = append(response, item.resp)
	}

	slices.SortFunc(response, func(a, b *ModemResponse) int {
		return strings.Compare(a.ID, b.ID)
	})
	return response, nil
}

func (c *catalog) Get(modem *mmodem.Modem) (*ModemResponse, error) {
	resp, err := c.buildResponse(modem)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *catalog) buildResponse(device *mmodem.Modem) (*ModemResponse, error) {
	sim, err := device.SIMs().Primary()
	if err != nil {
		slog.Error("failed to fetch SIM", "modem", device.EquipmentIdentifier, "error", err)
		return nil, err
	}

	percent, _, err := device.SignalQuality()
	if err != nil {
		slog.Error("failed to fetch signal quality", "modem", device.EquipmentIdentifier, "error", err)
		return nil, err
	}

	access, err := device.AccessTechnologies()
	if err != nil {
		slog.Error("failed to fetch access technologies", "modem", device.EquipmentIdentifier, "error", err)
		return nil, err
	}

	threeGpp := device.ThreeGPP()
	registrationState, err := threeGpp.RegistrationState()
	if err != nil {
		slog.Error("failed to fetch registration state", "modem", device.EquipmentIdentifier, "error", err)
		return nil, err
	}

	registeredOperatorName, err := threeGpp.OperatorName()
	if err != nil {
		slog.Error("failed to fetch operator name", "modem", device.EquipmentIdentifier, "error", err)
		return nil, err
	}

	operatorCode, err := threeGpp.OperatorCode()
	if err != nil {
		slog.Error("failed to fetch operator code", "modem", device.EquipmentIdentifier, "error", err)
		return nil, err
	}

	carrierInfo := carrier.Lookup(sim.OperatorIdentifier)
	supportsEsim, err := c.supportsEsim(device, sim)
	if err != nil {
		slog.Error("failed to detect eSIM support", "modem", device.EquipmentIdentifier, "error", err)
		return nil, err
	}

	simSlots, err := c.buildSlotsResponse(device)
	if err != nil {
		slog.Error("failed to fetch SIM slots", "modem", device.EquipmentIdentifier, "error", err)
		return nil, err
	}

	alias := c.cfg.FindModem(device.EquipmentIdentifier).Alias
	name := device.Model
	if alias != "" {
		name = alias
	}
	simOperatorName := carrierInfo.Name
	if sim.OperatorName != "" {
		simOperatorName = sim.OperatorName
	}
	return &ModemResponse{
		Manufacturer:     device.Manufacturer,
		ID:               device.EquipmentIdentifier,
		FirmwareRevision: device.FirmwareRevision,
		HardwareRevision: device.HardwareRevision,
		Name:             name,
		Number:           device.Number,
		SIM: SlotResponse{
			Active:             sim.Active,
			OperatorName:       simOperatorName,
			OperatorIdentifier: sim.OperatorIdentifier,
			RegionCode:         carrierInfo.Region,
			Identifier:         sim.Identifier,
		},
		Slots:             simSlots,
		AccessTechnology:  accessTechnologyString(access),
		RegistrationState: registrationState.String(),
		RegisteredOperator: RegisteredOperatorResponse{
			Name: registeredOperatorName,
			Code: operatorCode,
		},
		SignalQuality: percent,
		SupportsEsim:  supportsEsim,
	}, nil
}

func (c *catalog) buildSlotsResponse(device *mmodem.Modem) ([]SlotResponse, error) {
	if len(device.SimSlots) == 0 {
		return []SlotResponse{}, nil
	}
	simSlots := make([]SlotResponse, 0, len(device.SimSlots))
	for _, slotPath := range device.SimSlots {
		sim, err := device.SIMs().Get(slotPath)
		if err != nil {
			slog.Error("failed to fetch SIM for slot", "modem", device.EquipmentIdentifier, "slot", slotPath, "error", err)
			return nil, err
		}
		carrierInfo := carrier.Lookup(sim.OperatorIdentifier)
		operatorName := carrierInfo.Name
		if sim.OperatorName != "" {
			operatorName = sim.OperatorName
		}
		simSlots = append(simSlots, SlotResponse{
			Active:             sim.Active,
			OperatorName:       operatorName,
			OperatorIdentifier: sim.OperatorIdentifier,
			RegionCode:         carrierInfo.Region,
			Identifier:         sim.Identifier,
		})
	}
	return simSlots, nil
}

func (c *catalog) supportsEsim(m *mmodem.Modem, sim *mmodem.SIM) (bool, error) {
	if sim != nil && strings.TrimSpace(sim.Eid) != "" {
		return true, nil
	}
	if supported, ok := c.cachedEsimSupport(m.EquipmentIdentifier); ok {
		return supported, nil
	}
	supported, err := probeEsimSupport(m, c.cfg)
	if err != nil {
		return false, err
	}
	c.storeEsimSupport(m.EquipmentIdentifier, supported)
	return supported, nil
}

func (c *catalog) cachedEsimSupport(id string) (bool, bool) {
	c.mu.RLock()
	supported, ok := c.esim[id]
	c.mu.RUnlock()
	if !ok {
		return false, false
	}
	return supported, true
}

func (c *catalog) storeEsimSupport(id string, supported bool) {
	c.mu.Lock()
	c.esim[id] = supported
	c.mu.Unlock()
}

func probeEsimSupport(m *mmodem.Modem, cfg *config.Config) (bool, error) {
	client, err := lpa.New(m, cfg)
	if err != nil {
		if errors.Is(err, lpa.ErrNoSupportedAID) {
			return false, nil
		}
		slog.Error("failed to create LPA client", "modem", m.EquipmentIdentifier, "error", err)
		return false, err
	}
	defer func() {
		if cerr := client.Close(); cerr != nil {
			slog.Warn("failed to close LPA client", "error", cerr)
		}
	}()
	return true, nil
}

func accessTechnologyString(access []mmodem.ModemAccessTechnology) string {
	if len(access) == 0 {
		return ""
	}
	priority := []mmodem.ModemAccessTechnology{
		mmodem.ModemAccessTechnology5GNR,
		mmodem.ModemAccessTechnologyLte,
		mmodem.ModemAccessTechnologyLteCatM,
		mmodem.ModemAccessTechnologyLteNBIot,
		mmodem.ModemAccessTechnologyHspaPlus,
		mmodem.ModemAccessTechnologyHspa,
		mmodem.ModemAccessTechnologyHsupa,
		mmodem.ModemAccessTechnologyHsdpa,
		mmodem.ModemAccessTechnologyUmts,
		mmodem.ModemAccessTechnologyEdge,
		mmodem.ModemAccessTechnologyGprs,
		mmodem.ModemAccessTechnologyGsm,
		mmodem.ModemAccessTechnologyGsmCompact,
		mmodem.ModemAccessTechnologyEvdob,
		mmodem.ModemAccessTechnologyEvdoa,
		mmodem.ModemAccessTechnologyEvdo0,
		mmodem.ModemAccessTechnology1xrtt,
		mmodem.ModemAccessTechnologyPots,
	}
	for _, tech := range priority {
		if slices.Contains(access, tech) {
			return tech.String()
		}
	}
	return access[0].String()
}
