//go:build linux

package computecloud

// This is an offline-only CUDA protocol experiment. No HTTP route, marketplace,
// wallet, signing, proofs, payments or persistent job state are used here.

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	"prism/internal/usefulwork"
)

const (
	cloudGPUMaxStdoutV1     = 128 * 1024
	cloudGPUMaxStderrV1     = 16 * 1024
	cloudGPUMaxIPCRequestV1 = 512 * 1024
	cloudGPUTimeoutV1       = 20 * time.Second
)

// CloudGPUResultV1 is a verified *result*, not a proof or hardware attestation.
// It is deliberately unsuitable for marketplace settlement.
type CloudGPUResultV1 struct {
	Mode            string   `json:"mode"`
	TaskID          string   `json:"task_id"`
	InputHash       string   `json:"input_hash"`
	ResultValues    []uint64 `json:"result_values"`
	OutputHash      string   `json:"output_hash"`
	CPUVerified     bool     `json:"cpu_verified"`
	GPUAttestation  bool     `json:"gpu_attestation"`
	DispatchEnabled bool     `json:"dispatch_enabled"`
	ProofsAvailable bool     `json:"proofs_available"`
}

type cloudGPUFrameV1 struct {
	Schema  string          `json:"schema"`
	Seq     int             `json:"seq"`
	Nonce   string          `json:"nonce"`
	Receipt json.RawMessage `json:"receipt"`
}

type cloudGPUReceiptV1 struct {
	Schema         string   `json:"schema"`
	TaskID         string   `json:"task_id"`
	InputHash      string   `json:"input_hash"`
	ResultValues   []uint64 `json:"result_values"`
	OutputHash     string   `json:"output_hash"`
	GPUAttestation bool     `json:"gpu_attestation"`
}

// validateCloudGPUBinaryV1 pins the exact lab binary. No symlinks are accepted.
// A hash is an integrity check, not process sandboxing or hardware attestation.
func validateCloudGPUBinaryV1(path, expectedHash string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("GPU binary must use a clean absolute path")
	}
	if len(expectedHash) != 64 || expectedHash != hex.EncodeToString(mustHex64V1(expectedHash)) {
		return errors.New("expected GPU binary hash must be 64 lowercase hex characters")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("GPU binary unavailable: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 || info.Mode().Perm()&0022 != 0 {
		return errors.New("GPU binary must be an executable regular file without group/other write permissions")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, io.LimitReader(f, 256*1024*1024+1)); err != nil {
		return err
	}
	if fi, err := f.Stat(); err != nil {
		return err
	} else if fi.Size() > 256*1024*1024 {
		return errors.New("GPU binary size exceeded")
	}
	if subtle.ConstantTimeCompare([]byte(hex.EncodeToString(h.Sum(nil))), []byte(expectedHash)) != 1 {
		return errors.New("GPU binary SHA-256 mismatch")
	}
	return nil
}

func mustHex64V1(s string) []byte {
	raw, err := hex.DecodeString(s)
	if err != nil || len(raw) != 32 {
		return nil
	}
	return raw
}

type cloudGPUCaptureV1 struct {
	data []byte
	max  int
}

func (c *cloudGPUCaptureV1) Write(p []byte) (int, error) {
	if len(p) > c.max-len(c.data) {
		return 0, errors.New("GPU process output limit exceeded")
	}
	c.data = append(c.data, p...)
	return len(p), nil
}

// strictCloudGPUObjectV1 rejects duplicate, missing, or extra keys.
func strictCloudGPUObjectV1(raw []byte, required []string, dest any) error {
	allowed := make(map[string]bool, len(required))
	for _, name := range required {
		allowed[name] = false
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	start, err := dec.Token()
	if err != nil || start != json.Delim('{') {
		return errors.New("GPU frame must be JSON object")
	}
	for dec.More() {
		v, err := dec.Token()
		if err != nil {
			return err
		}
		key, ok := v.(string)
		if !ok {
			return errors.New("invalid GPU object key")
		}
		present, allowedKey := allowed[key]
		if !allowedKey || present {
			return errors.New("GPU object has extra or duplicate key")
		}
		allowed[key] = true
		var value json.RawMessage
		if err = dec.Decode(&value); err != nil {
			return err
		}
	}
	end, err := dec.Token()
	if err != nil || end != json.Delim('}') {
		return errors.New("GPU object missing end delimiter")
	}
	var extra any
	if err = dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("GPU JSON has trailing content")
	}
	for _, present := range allowed {
		if !present {
			return errors.New("GPU object missing required key")
		}
	}
	strict := json.NewDecoder(bytes.NewReader(raw))
	strict.DisallowUnknownFields()
	return strict.Decode(dest)
}

func parseCloudGPUFrameV1(task usefulwork.Task, expectedNonce string, raw []byte, reference []uint64) (CloudGPUResultV1, error) {
	fail := func(err error) (CloudGPUResultV1, error) { return CloudGPUResultV1{}, err }
	prefix := []byte("PRISM_GPU_SESSION=")
	if len(raw) > cloudGPUMaxStdoutV1 || !bytes.HasPrefix(raw, prefix) ||
		len(raw) == 0 || raw[len(raw)-1] != '\n' || bytes.Count(raw, []byte{'\n'}) != 1 || bytes.Contains(raw, []byte{'\r'}) {
		return fail(errors.New("GPU process must emit exactly one bounded newline-terminated frame"))
	}
	var frame cloudGPUFrameV1
	if err := strictCloudGPUObjectV1(raw[len(prefix):len(raw)-1], []string{"schema", "seq", "nonce", "receipt"}, &frame); err != nil {
		return fail(err)
	}
	if frame.Schema != "prism-gpu-session-frame-v063" || frame.Seq != 0 ||
		len(frame.Nonce) != 64 || len(expectedNonce) != 64 ||
		subtle.ConstantTimeCompare([]byte(frame.Nonce), []byte(expectedNonce)) != 1 {
		return fail(errors.New("GPU session schema, seq, or nonce mismatch"))
	}
	var receipt cloudGPUReceiptV1
	if err := strictCloudGPUObjectV1(frame.Receipt, []string{"schema", "task_id", "input_hash", "result_values", "output_hash", "gpu_attestation"}, &receipt); err != nil {
		return fail(err)
	}
	if receipt.Schema != "prism-gpu-ml-lab-v0" || receipt.GPUAttestation ||
		receipt.TaskID != task.ID || receipt.InputHash != task.InputHash {
		return fail(errors.New("GPU receipt commitment mismatch"))
	}
	if uint64(len(receipt.ResultValues)) != task.RowsA || !slices.Equal(receipt.ResultValues, reference) {
		return fail(errors.New("GPU result differs from independent CPU reference"))
	}
	out, err := json.Marshal(receipt.ResultValues)
	if err != nil {
		return fail(err)
	}
	hash := sha256.Sum256(out)
	if receipt.OutputHash != hex.EncodeToString(hash[:]) {
		return fail(errors.New("GPU output hash mismatch"))
	}
	return CloudGPUResultV1{
		Mode: "offline_gpu_lab", TaskID: task.ID, InputHash: task.InputHash,
		ResultValues: append([]uint64(nil), receipt.ResultValues...), OutputHash: receipt.OutputHash,
		CPUVerified: true, GPUAttestation: false, DispatchEnabled: false, ProofsAvailable: false,
	}, nil
}

// RunLocalGPULabV1 launches one short-lived, pinned CUDA process.
// Explicit CLI-only call. It does NOT contact Prism Core, Cloud HTTP, wallets or marketplace.
func RunLocalGPULabV1(ctx context.Context, body []byte, gpuBinary, expectedSHA256 string) (CloudGPUResultV1, error) {
	return runLocalGPULabV1(ctx, body, gpuBinary, expectedSHA256, "")
}

// RunLocalGPULabWithGlobalLockV1 is an opt-in offline executor whose child
// inherits the flock open-file description on fd 3. The supervisor only
// closes its own descriptor, never unlocks the inherited flock.
func RunLocalGPULabWithGlobalLockV1(ctx context.Context, body []byte, gpuBinary, expectedSHA256, lockDir string) (CloudGPUResultV1, error) {
	if lockDir == "" {
		return CloudGPUResultV1{}, errors.New("GPU global lock directory required")
	}
	return runLocalGPULabV1(ctx, body, gpuBinary, expectedSHA256, lockDir)
}

func runLocalGPULabV1(ctx context.Context, body []byte, gpuBinary, expectedSHA256, lockDir string) (CloudGPUResultV1, error) {
	var empty CloudGPUResultV1
	if ctx == nil {
		return empty, errors.New("nil context")
	}
	task, err := ParseSubmitV1(body)
	if err != nil {
		return empty, err
	}
	reference, err := usefulwork.ComputeMLInferenceQuantized(task)
	if err != nil {
		return empty, err
	}
	if err := validateCloudGPUBinaryV1(gpuBinary, expectedSHA256); err != nil {
		return empty, err
	}
	nonceBytes := make([]byte, 32)
	if _, err = rand.Read(nonceBytes); err != nil {
		return empty, err
	}
	nonce := hex.EncodeToString(nonceBytes)
	request, err := json.Marshal(struct {
		Seq   int             `json:"seq"`
		Nonce string          `json:"nonce"`
		Task  usefulwork.Task `json:"task"`
	}{Seq: 0, Nonce: nonce, Task: task})
	if err != nil {
		return empty, err
	}
	if len(request)+1 > cloudGPUMaxIPCRequestV1 {
		return empty, errors.New("GPU request size limit exceeded")
	}
	request = append(request, '\n')

	session, cancel := context.WithTimeout(ctx, cloudGPUTimeoutV1)
	defer cancel()
	// Acquire only after workload validation and executable pinning.
	// The inherited descriptor protects an orphaned pinned worker when
	// its supervisor is killed. This is cooperative, not GPU isolation.
	var gpuLock *GlobalGPULockV1
	if lockDir != "" {
		gpuLock, err = TryAcquireGlobalGPULockV1(lockDir)
		if err != nil {
			return empty, fmt.Errorf("GPU global reservation refused: %w", err)
		}
		defer func() { _ = gpuLock.ClosePreservingInheritedLockV1() }()
	}
	cmd := exec.CommandContext(session, gpuBinary)
	if gpuLock != nil {
		cmd.ExtraFiles = []*os.File{gpuLock.file}
	}
	cmd.Stdin = bytes.NewReader(request)
	stdout := &cloudGPUCaptureV1{max: cloudGPUMaxStdoutV1}
	stderr := &cloudGPUCaptureV1{max: cloudGPUMaxStderrV1}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	// Dedicated process group: interruption also kills subprocess descendants.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = 2 * time.Second
	// Whitelist only basic runtime environment: do not pass API keys or Cloud config.
	home, err := os.UserHomeDir()
	if err != nil {
		return empty, err
	}
	cmd.Env = []string{
		"HOME=" + home,
		"PATH=/usr/local/cuda-13.2/bin:/usr/local/bin:/usr/bin:/bin",
		"CUDA_HOME=/usr/local/cuda-13.2",
		"CUDA_TOOLKIT_PATH=/usr/local/cuda-13.2",
		"LC_ALL=C",
	}
	if err = cmd.Run(); err != nil {
		if session.Err() != nil {
			return empty, fmt.Errorf("GPU session timeout/cancel: %w", session.Err())
		}
		return empty, fmt.Errorf("GPU worker failed (no result accepted): %w", err)
	}
	if session.Err() != nil {
		return empty, session.Err()
	}
	return parseCloudGPUFrameV1(task, nonce, stdout.data, reference)
}
