package crosschain

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const settlementFilename = "crosschain-settlements.json"

const (
	SettlementChainArbitrum = "arbitrum"
	SettlementChainSolana   = "solana"

	SettlementStatusPending   = "pending"
	SettlementStatusConfirmed = "confirmed"
	SettlementStatusFailed    = "failed"
)

var evmAddressPattern = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)

type Settlement struct {
	RegistryID      string `json:"registryId"`
	Chain           string `json:"chain"`
	Status          string `json:"status"`
	TxHash          string `json:"txHash,omitempty"`
	RegistryAddress string `json:"registryAddress,omitempty"`
	BlockNumber     uint64 `json:"blockNumber,omitempty"`
	ExplorerURL     string `json:"explorerUrl,omitempty"`
	UpdatedAt       string `json:"updatedAt"`
}

type settlementSnapshot struct {
	Version     uint64       `json:"version"`
	Settlements []Settlement `json:"settlements"`
}

type SettlementStore struct {
	mu      sync.RWMutex
	dataDir string
	records map[string]Settlement
}

func NewSettlementStore(
	dataDir string,
) (*SettlementStore, error) {
	dataDir = strings.TrimSpace(dataDir)

	if dataDir == "" {
		return nil, fmt.Errorf(
			"cross-chain settlement data directory cannot be empty",
		)
	}

	store := &SettlementStore{
		dataDir: dataDir,
		records: make(map[string]Settlement),
	}

	if err := store.load(); err != nil {
		return nil, err
	}

	return store, nil
}

func settlementKey(
	registryID string,
	chain string,
) string {
	return strings.ToLower(
		strings.TrimSpace(registryID),
	) + "|" + strings.ToLower(
		strings.TrimSpace(chain),
	)
}

func validateSettlement(
	value Settlement,
) error {
	if _, err := decodeBytes32(
		value.RegistryID,
	); err != nil {
		return fmt.Errorf(
			"invalid registry ID: %w",
			err,
		)
	}

	switch value.Chain {
	case SettlementChainArbitrum,
		SettlementChainSolana:
	default:
		return fmt.Errorf(
			"unsupported settlement chain: %s",
			value.Chain,
		)
	}

	switch value.Status {
	case SettlementStatusPending,
		SettlementStatusConfirmed,
		SettlementStatusFailed:
	default:
		return fmt.Errorf(
			"unsupported settlement status: %s",
			value.Status,
		)
	}

	if value.Status ==
		SettlementStatusConfirmed {

		if strings.TrimSpace(
			value.TxHash,
		) == "" {
			return fmt.Errorf(
				"confirmed settlement requires transaction hash",
			)
		}

		if value.BlockNumber == 0 {
			return fmt.Errorf(
				"confirmed settlement requires block number",
			)
		}
	}

	if value.Chain ==
		SettlementChainArbitrum {

		if value.TxHash != "" {
			if _, err := decodeBytes32(
				value.TxHash,
			); err != nil {
				return fmt.Errorf(
					"invalid Arbitrum transaction hash: %w",
					err,
				)
			}
		}

		if value.RegistryAddress != "" &&
			!evmAddressPattern.MatchString(
				value.RegistryAddress,
			) {

			return fmt.Errorf(
				"invalid Arbitrum registry address",
			)
		}
	}

	return nil
}

func (store *SettlementStore) Upsert(
	value Settlement,
) (Settlement, error) {
	value.RegistryID =
		strings.ToLower(
			strings.TrimSpace(
				value.RegistryID,
			),
		)

	value.Chain =
		strings.ToLower(
			strings.TrimSpace(
				value.Chain,
			),
		)

	value.Status =
		strings.ToLower(
			strings.TrimSpace(
				value.Status,
			),
		)

	value.TxHash =
		strings.TrimSpace(
			value.TxHash,
		)

	value.RegistryAddress =
		strings.TrimSpace(
			value.RegistryAddress,
		)

	value.ExplorerURL =
		strings.TrimSpace(
			value.ExplorerURL,
		)

	value.UpdatedAt =
		time.Now().
			UTC().
			Format(time.RFC3339)

	if err := validateSettlement(
		value,
	); err != nil {
		return Settlement{}, err
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	store.records[settlementKey(
		value.RegistryID,
		value.Chain,
	)] = value

	if err := store.persistLocked(); err != nil {
		return Settlement{}, err
	}

	return value, nil
}

func (store *SettlementStore) ForRegistry(
	registryID string,
) ([]Settlement, error) {
	if _, err := decodeBytes32(
		registryID,
	); err != nil {
		return nil, fmt.Errorf(
			"invalid registry ID: %w",
			err,
		)
	}

	registryID =
		strings.ToLower(
			strings.TrimSpace(registryID),
		)

	store.mu.RLock()
	defer store.mu.RUnlock()

	result := make(
		[]Settlement,
		0,
	)

	for _, settlement := range store.records {

		if settlement.RegistryID ==
			registryID {

			result = append(
				result,
				settlement,
			)
		}
	}

	sort.Slice(
		result,
		func(i, j int) bool {
			return result[i].Chain <
				result[j].Chain
		},
	)

	return result, nil
}

func (store *SettlementStore) load() error {
	path := filepath.Join(
		store.dataDir,
		settlementFilename,
	)

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(
			err,
			os.ErrNotExist,
		) {
			return nil
		}

		return fmt.Errorf(
			"cannot load cross-chain settlements: %w",
			err,
		)
	}

	var snapshot settlementSnapshot

	if err := json.Unmarshal(
		data,
		&snapshot,
	); err != nil {
		return fmt.Errorf(
			"cannot decode cross-chain settlements: %w",
			err,
		)
	}

	if snapshot.Version != 1 {
		return fmt.Errorf(
			"unsupported cross-chain settlement version: %d",
			snapshot.Version,
		)
	}

	for _, settlement := range snapshot.Settlements {

		if err := validateSettlement(
			settlement,
		); err != nil {
			return fmt.Errorf(
				"invalid persisted cross-chain settlement: %w",
				err,
			)
		}

		store.records[settlementKey(
			settlement.RegistryID,
			settlement.Chain,
		)] = settlement
	}

	return nil
}

func (store *SettlementStore) persistLocked() error {
	if err := os.MkdirAll(
		store.dataDir,
		0755,
	); err != nil {
		return err
	}

	values := make(
		[]Settlement,
		0,
		len(store.records),
	)

	for _, settlement := range store.records {

		values = append(
			values,
			settlement,
		)
	}

	sort.Slice(
		values,
		func(i, j int) bool {
			if values[i].RegistryID ==
				values[j].RegistryID {

				return values[i].Chain <
					values[j].Chain
			}

			return values[i].RegistryID <
				values[j].RegistryID
		},
	)

	data, err := json.MarshalIndent(
		settlementSnapshot{
			Version:     1,
			Settlements: values,
		},
		"",
		"  ",
	)
	if err != nil {
		return fmt.Errorf(
			"cannot encode cross-chain settlements: %w",
			err,
		)
	}

	temp, err := os.CreateTemp(
		store.dataDir,
		".crosschain-settlements-*.tmp",
	)
	if err != nil {
		return fmt.Errorf(
			"cannot create settlement temp file: %w",
			err,
		)
	}

	tempPath := temp.Name()

	defer func() {
		_ = os.Remove(tempPath)
	}()

	if err := temp.Chmod(0600); err != nil {
		_ = temp.Close()
		return err
	}

	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}

	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}

	if err := temp.Close(); err != nil {
		return err
	}

	path := filepath.Join(
		store.dataDir,
		settlementFilename,
	)

	if err := os.Rename(
		tempPath,
		path,
	); err != nil {
		return fmt.Errorf(
			"cannot replace settlement state: %w",
			err,
		)
	}

	return nil
}
