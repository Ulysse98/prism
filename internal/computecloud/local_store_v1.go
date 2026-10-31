//go:build linux

package computecloud

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"
)

const MaxJobsPerTenantV1 = 32

var (
	ErrCloudConflict = errors.New("idempotency conflict")
	ErrCloudQuota    = errors.New("tenant job quota exceeded")
	ErrCloudNotFound = errors.New("job not found")
)

type CloudJobV1 struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"-"`
	TaskID         string    `json:"task_id"`
	InputHash      string    `json:"input_hash"`
	IdempotencyKey string    `json:"-"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

type diskJobV1 struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id"`
	TaskID         string    `json:"task_id"`
	InputHash      string    `json:"input_hash"`
	IdempotencyKey string    `json:"idempotency_key"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

type diskSnapshotV1 struct {
	Version int         `json:"version"`
	Jobs    []diskJobV1 `json:"jobs"`
}

type LocalCloudStoreV1 struct {
	mu     sync.Mutex
	path   string
	lock   *os.File
	jobs   map[string]diskJobV1
	dedupe map[string]string
}

func OpenLocalCloudStoreV1(path string) (*LocalCloudStoreV1, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("store path must be absolute")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("store directory must be private (0700)")
	}
	lockPath := path + ".lock"
	if st, err := os.Lstat(lockPath); err == nil && !st.Mode().IsRegular() {
		return nil, errors.New("lock path is not a regular file")
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		return nil, fmt.Errorf("store already open: %w", err)
	}
	s := &LocalCloudStoreV1{path: path, lock: lock, jobs: make(map[string]diskJobV1), dedupe: make(map[string]string)}
	if err := s.load(); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

func (s *LocalCloudStoreV1) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lock == nil {
		return nil
	}
	err := syscall.Flock(int(s.lock.Fd()), syscall.LOCK_UN)
	cerr := s.lock.Close()
	s.lock = nil
	if err != nil {
		return err
	}
	return cerr
}
func cloudKey(tenant, idem string) string { return tenant + "\x00" + idem }
func (s *LocalCloudStoreV1) load() error {
	st, err := os.Lstat(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() || st.Mode().Perm()&0077 != 0 || st.Size() > 16*1024*1024 {
		return errors.New("store file must be private, regular and bounded")
	}
	f, err := os.Open(s.path)
	if err != nil {
		return err
	}
	defer f.Close()
	var snapshot diskSnapshotV1
	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&snapshot); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("unexpected trailing store data")
	}
	if snapshot.Version != 1 {
		return errors.New("unsupported store version")
	}
	if len(snapshot.Jobs) > 100000 {
		return errors.New("store oversized")
	}
	for _, j := range snapshot.Jobs {
		if j.ID == "" || j.TenantID == "" || j.IdempotencyKey == "" || j.TaskID == "" || j.Status != "SIMULATED" {
			return errors.New("invalid stored job")
		}
		if _, ok := s.jobs[j.ID]; ok {
			return errors.New("duplicate stored job id")
		}
		dk := cloudKey(j.TenantID, j.IdempotencyKey)
		if _, ok := s.dedupe[dk]; ok {
			return errors.New("duplicate stored idempotency key")
		}
		s.jobs[j.ID] = j
		s.dedupe[dk] = j.ID
	}
	return nil
}
func (s *LocalCloudStoreV1) Count(tenant string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, j := range s.jobs {
		if j.TenantID == tenant {
			n++
		}
	}
	return n
}
func (s *LocalCloudStoreV1) Get(tenant, id string) (CloudJobV1, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok || j.TenantID != tenant {
		return CloudJobV1{}, ErrCloudNotFound
	}
	return publicCloudJob(j), nil
}
func publicCloudJob(j diskJobV1) CloudJobV1 {
	return CloudJobV1{ID: j.ID, TaskID: j.TaskID, InputHash: j.InputHash, Status: j.Status, CreatedAt: j.CreatedAt}
}
func (s *LocalCloudStoreV1) Create(tenant, idem, taskID, inputHash string) (CloudJobV1, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lock == nil {
		return CloudJobV1{}, false, errors.New("store closed")
	}
	dk := cloudKey(tenant, idem)
	if id, ok := s.dedupe[dk]; ok {
		j := s.jobs[id]
		if j.TaskID != taskID || j.InputHash != inputHash {
			return CloudJobV1{}, false, ErrCloudConflict
		}
		return publicCloudJob(j), false, nil
	}
	used := 0
	for _, j := range s.jobs {
		if j.TenantID == tenant {
			used++
		}
	}
	if used >= MaxJobsPerTenantV1 {
		return CloudJobV1{}, false, ErrCloudQuota
	}
	var idBytes [16]byte
	if _, err := rand.Read(idBytes[:]); err != nil {
		return CloudJobV1{}, false, err
	}
	j := diskJobV1{ID: "job_" + hex.EncodeToString(idBytes[:]), TenantID: tenant, IdempotencyKey: idem, TaskID: taskID, InputHash: inputHash, Status: "SIMULATED", CreatedAt: time.Now().UTC()}
	candidate := make([]diskJobV1, 0, len(s.jobs)+1)
	for _, x := range s.jobs {
		candidate = append(candidate, x)
	}
	candidate = append(candidate, j)
	sort.Slice(candidate, func(a, b int) bool { return candidate[a].ID < candidate[b].ID })
	if err := s.persist(candidate); err != nil {
		return CloudJobV1{}, false, err
	}
	s.jobs[j.ID] = j
	s.dedupe[dk] = j.ID
	return publicCloudJob(j), true, nil
}
func (s *LocalCloudStoreV1) persist(jobs []diskJobV1) error {
	dir := filepath.Dir(s.path)
	f, err := os.CreateTemp(dir, ".jobs-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if err = json.NewEncoder(f).Encode(diskSnapshotV1{Version: 1, Jobs: jobs}); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmp, s.path); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
