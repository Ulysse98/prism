package compute

import (
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	"prism/internal/usefulwork"
)

type Marketplace struct {
	mu      sync.RWMutex
	jobs    map[string]Job
	dataDir string
	db      *sql.DB
}

func NewMarketplace() *Marketplace {
	return &Marketplace{
		jobs: make(map[string]Job),
	}
}

func (market *Marketplace) Create(
	task usefulwork.Task,
	requester string,
	reward uint64,
	nonce uint64,
) (Job, error) {

	if market == nil {
		return Job{}, fmt.Errorf(
			"compute marketplace cannot be nil",
		)
	}

	job, err := NewJob(
		task,
		requester,
		reward,
		nonce,
	)
	if err != nil {
		return Job{}, err
	}

	market.mu.Lock()
	defer market.mu.Unlock()

	if _, exists := market.jobs[job.ID]; exists {
		return Job{}, fmt.Errorf(
			"compute job already exists",
		)
	}

	market.jobs[job.ID] = job

	if err := market.persistLocked(); err != nil {
		delete(market.jobs, job.ID)

		return Job{}, fmt.Errorf(
			"cannot persist compute job: %w",
			err,
		)
	}

	return job, nil
}

func (market *Marketplace) Get(
	jobID string,
) (Job, error) {

	if market == nil {
		return Job{}, fmt.Errorf(
			"compute marketplace cannot be nil",
		)
	}

	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return Job{}, fmt.Errorf(
			"compute job ID cannot be empty",
		)
	}

	market.mu.RLock()
	defer market.mu.RUnlock()

	job, exists := market.jobs[jobID]
	if !exists {
		return Job{}, fmt.Errorf(
			"compute job not found",
		)
	}

	return job, nil
}

func (market *Marketplace) List() []Job {
	if market == nil {
		return nil
	}

	market.mu.RLock()
	defer market.mu.RUnlock()

	return market.listLocked()
}

func (market *Marketplace) listLocked() []Job {
	jobs := make(
		[]Job,
		0,
		len(market.jobs),
	)

	for _, job := range market.jobs {
		jobs = append(
			jobs,
			job,
		)
	}

	sort.Slice(
		jobs,
		func(i, j int) bool {
			return jobs[i].ID < jobs[j].ID
		},
	)

	return jobs
}

func (market *Marketplace) Claim(
	jobID string,
	worker string,
) (Job, error) {

	if market == nil {
		return Job{}, fmt.Errorf(
			"compute marketplace cannot be nil",
		)
	}

	jobID = strings.TrimSpace(jobID)

	market.mu.Lock()
	defer market.mu.Unlock()

	job, exists := market.jobs[jobID]
	if !exists {
		return Job{}, fmt.Errorf(
			"compute job not found",
		)
	}

	previous := job

	if err := job.Claim(worker); err != nil {
		return Job{}, err
	}

	market.jobs[jobID] = job

	if err := market.persistLocked(); err != nil {
		market.jobs[jobID] = previous

		return Job{}, fmt.Errorf(
			"cannot persist compute job claim: %w",
			err,
		)
	}

	return job, nil
}

func (market *Marketplace) Complete(
	jobID string,
	proof usefulwork.Proof,
) (Job, error) {

	if market == nil {
		return Job{}, fmt.Errorf(
			"compute marketplace cannot be nil",
		)
	}

	jobID = strings.TrimSpace(jobID)

	market.mu.Lock()
	defer market.mu.Unlock()

	job, exists := market.jobs[jobID]
	if !exists {
		return Job{}, fmt.Errorf(
			"compute job not found",
		)
	}

	previous := job

	if err := job.Complete(proof); err != nil {
		return Job{}, err
	}

	market.jobs[jobID] = job

	if err := market.persistLocked(); err != nil {
		market.jobs[jobID] = previous

		return Job{}, fmt.Errorf(
			"cannot persist compute job completion: %w",
			err,
		)
	}

	return job, nil
}
func (market *Marketplace) ReservedRewardFor(
	requester string,
) (uint64, error) {

	if market == nil {
		return 0, fmt.Errorf(
			"compute marketplace cannot be nil",
		)
	}

	requester = strings.TrimSpace(requester)
	if requester == "" {
		return 0, fmt.Errorf(
			"compute requester cannot be empty",
		)
	}

	market.mu.RLock()
	defer market.mu.RUnlock()

	var reserved uint64

	for _, job := range market.jobs {
		if !strings.EqualFold(job.Requester, requester) {
			continue
		}

		if job.Status != JobStatusOpen &&
			job.Status != JobStatusClaimed {
			continue
		}

		if reserved > math.MaxUint64-job.Reward {
			return 0, fmt.Errorf(
				"compute reserved reward overflow for requester %s",
				requester,
			)
		}

		reserved += job.Reward
	}

	return reserved, nil
}
