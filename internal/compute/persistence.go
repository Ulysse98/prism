package compute

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"prism/internal/usefulwork"
)

const marketplaceFilename = "compute-jobs.json"

type marketplaceSnapshot struct {
	Version uint64 `json:"version"`
	Jobs    []Job  `json:"jobs"`
}

func NewPersistentMarketplace(
	dataDir string,
) (*Marketplace, error) {

	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return nil, fmt.Errorf(
			"compute marketplace data directory cannot be empty",
		)
	}

	market := &Marketplace{
		jobs:    make(map[string]Job),
		dataDir: dataDir,
	}

	if err := market.load(); err != nil {
		return nil, err
	}

	return market, nil
}

func (market *Marketplace) load() error {
	path := filepath.Join(
		market.dataDir,
		marketplaceFilename,
	)

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return fmt.Errorf(
			"cannot load compute marketplace: %w",
			err,
		)
	}

	var snapshot marketplaceSnapshot

	if err := json.Unmarshal(
		data,
		&snapshot,
	); err != nil {
		return fmt.Errorf(
			"cannot decode compute marketplace: %w",
			err,
		)
	}

	if snapshot.Version != 1 {
		return fmt.Errorf(
			"unsupported compute marketplace version: %d",
			snapshot.Version,
		)
	}

	for _, job := range snapshot.Jobs {
		if err := validatePersistedJob(job); err != nil {
			return fmt.Errorf(
				"invalid persisted compute job %s: %w",
				job.ID,
				err,
			)
		}

		if _, exists := market.jobs[job.ID]; exists {
			return fmt.Errorf(
				"duplicate persisted compute job: %s",
				job.ID,
			)
		}

		market.jobs[job.ID] = job
	}

	return nil
}

func (market *Marketplace) persistLocked() error {
	if market.dataDir == "" {
		return nil
	}

	if err := os.MkdirAll(
		market.dataDir,
		0755,
	); err != nil {
		return err
	}

	snapshot := marketplaceSnapshot{
		Version: 1,
		Jobs:    market.listLocked(),
	}

	data, err := json.MarshalIndent(
		snapshot,
		"",
		"  ",
	)
	if err != nil {
		return fmt.Errorf(
			"cannot encode compute marketplace: %w",
			err,
		)
	}

	temp, err := os.CreateTemp(
		market.dataDir,
		".compute-jobs-*.tmp",
	)
	if err != nil {
		return fmt.Errorf(
			"cannot create compute marketplace temp file: %w",
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
		market.dataDir,
		marketplaceFilename,
	)

	if err := os.Rename(
		tempPath,
		path,
	); err != nil {
		return fmt.Errorf(
			"cannot replace compute marketplace state: %w",
			err,
		)
	}

	return nil
}

func validatePersistedJob(
	job Job,
) error {

	if err := usefulwork.ValidateTask(
		job.Task,
	); err != nil {
		return err
	}

	if strings.TrimSpace(job.Requester) == "" {
		return fmt.Errorf(
			"requester cannot be empty",
		)
	}

	if job.Reward == 0 {
		return fmt.Errorf(
			"reward must be greater than zero",
		)
	}

	workUnits, err := usefulwork.WorkUnits(
		job.Task,
	)
	if err != nil {
		return err
	}

	if job.WorkUnits != workUnits {
		return fmt.Errorf(
			"invalid work units",
		)
	}

	if job.ID != CalculateJobID(job) {
		return fmt.Errorf(
			"invalid compute job ID",
		)
	}

	switch job.Status {
	case JobStatusOpen:
		if job.Worker != "" ||
			job.ProofID != "" {

			return fmt.Errorf(
				"OPEN job contains completion state",
			)
		}

	case JobStatusClaimed:
		if strings.TrimSpace(job.Worker) == "" {
			return fmt.Errorf(
				"CLAIMED job has no worker",
			)
		}

		if job.ProofID != "" {
			return fmt.Errorf(
				"CLAIMED job contains proof ID",
			)
		}

	case JobStatusVerified:
		if strings.TrimSpace(job.Worker) == "" {
			return fmt.Errorf(
				"VERIFIED job has no worker",
			)
		}

		if strings.TrimSpace(job.ProofID) == "" {
			return fmt.Errorf(
				"VERIFIED job has no proof ID",
			)
		}

	default:
		return fmt.Errorf(
			"invalid compute job status: %s",
			job.Status,
		)
	}

	return nil
}
