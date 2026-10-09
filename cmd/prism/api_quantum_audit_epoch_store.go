package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	quantumAuditEpochJournalName     = "quantum-audit-epochs.jsonl"
	quantumAuditEpochJournalMaxBytes = 1 << 20
)

type quantumAuditEpochRecord struct {
	Epoch             uint64 `json:"epoch"`
	PolicyFingerprint string `json:"policy_fingerprint"`
}

func validateQuantumAuditEpochRecord(
	record quantumAuditEpochRecord,
) error {
	if record.Epoch == 0 {
		return fmt.Errorf("quantum audit epoch must be positive")
	}

	fingerprint := record.PolicyFingerprint

	if len(fingerprint) != 64 ||
		fingerprint != strings.ToLower(fingerprint) {
		return fmt.Errorf("invalid policy fingerprint")
	}

	decoded, err := hex.DecodeString(fingerprint)
	if err != nil || len(decoded) != 32 {
		return fmt.Errorf("invalid policy fingerprint encoding")
	}

	return nil
}

func readQuantumAuditEpochJournal(
	path string,
) (quantumAuditEpochRecord, bool, int64, error) {
	var empty quantumAuditEpochRecord

	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return empty, false, 0, nil
	}
	if err != nil {
		return empty, false, 0, err
	}

	// Reject symbolic links, directories and irregular files.
	if !info.Mode().IsRegular() {
		return empty, false, 0,
			fmt.Errorf("invalid epoch journal file type")
	}

	if info.Size() == 0 ||
		info.Size() > quantumAuditEpochJournalMaxBytes {
		return empty, false, 0,
			fmt.Errorf("invalid epoch journal size")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return empty, false, 0, err
	}

	if int64(len(data)) != info.Size() ||
		len(data) == 0 ||
		data[len(data)-1] != '\n' {
		return empty, false, 0,
			fmt.Errorf("truncated or changing epoch journal")
	}

	lines := bytes.Split(data[:len(data)-1], []byte("\n"))

	var last quantumAuditEpochRecord

	for index, line := range lines {
		if len(line) == 0 {
			return empty, false, 0,
				fmt.Errorf("empty epoch journal record")
		}

		decoder := json.NewDecoder(bytes.NewReader(line))
		decoder.DisallowUnknownFields()

		var record quantumAuditEpochRecord
		if err := decoder.Decode(&record); err != nil {
			return empty, false, 0,
				fmt.Errorf("invalid epoch record %d: %w",
					index+1, err)
		}

		var trailing any
		if err := decoder.Decode(&trailing); err != io.EOF {
			return empty, false, 0,
				fmt.Errorf("trailing data in epoch record")
		}

		if err := validateQuantumAuditEpochRecord(
			record,
		); err != nil {
			return empty, false, 0, err
		}

		if index > 0 && record.Epoch <= last.Epoch {
			return empty, false, 0,
				fmt.Errorf("non-monotonic epoch journal")
		}

		last = record
	}

	return last, true, int64(len(data)), nil
}

// recordQuantumAuditPolicyEpoch is intended to run once
// during single-process API startup, before serving requests.
//
// Existing history is never overwritten by this function.
// Corrupt or incomplete history fails closed.
func recordQuantumAuditPolicyEpoch(
	dataDir string,
	epoch uint64,
	fingerprint string,
) (returnErr error) {
	if strings.TrimSpace(dataDir) == "" {
		return fmt.Errorf("missing Prism node data directory")
	}

	next := quantumAuditEpochRecord{
		Epoch:             epoch,
		PolicyFingerprint: fingerprint,
	}

	if err := validateQuantumAuditEpochRecord(next); err != nil {
		return err
	}

	// Mkdir is atomic on local Windows and Unix filesystems.
	// A concurrent startup must fail instead of appending to
	// a stale snapshot of the journal.
	lockPath := filepath.Join(
		dataDir,
		quantumAuditEpochJournalName+".lock",
	)

	if err := os.Mkdir(lockPath, 0700); err != nil {
		return fmt.Errorf(
			"quantum audit epoch journal locked or unavailable: %w",
			err,
		)
	}

	defer func() {
		if err := os.Remove(lockPath); err != nil {
			returnErr = errors.Join(
				returnErr,
				fmt.Errorf("failed to release epoch lock: %w", err),
			)
		}
	}()

	path := filepath.Join(
		dataDir,
		quantumAuditEpochJournalName,
	)

	previous, exists, size, err :=
		readQuantumAuditEpochJournal(path)
	if err != nil {
		return err
	}

	if exists {
		if epoch < previous.Epoch {
			return fmt.Errorf(
				"policy epoch rollback: stored %d, requested %d",
				previous.Epoch, epoch,
			)
		}

		if epoch == previous.Epoch {
			if fingerprint != previous.PolicyFingerprint {
				return fmt.Errorf(
					"policy changed without advancing epoch",
				)
			}

			// An unchanged configuration is idempotent.
			return nil
		}
	}

	entry, err := json.Marshal(next)
	if err != nil {
		return err
	}

	entry = append(entry, '\n')

	if size+int64(len(entry)) >
		quantumAuditEpochJournalMaxBytes {
		return fmt.Errorf("epoch journal capacity exceeded")
	}

	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL

	if exists {
		flags = os.O_WRONLY | os.O_APPEND
	}

	file, err := os.OpenFile(path, flags, 0600)
	if err != nil {
		return err
	}

	written, writeErr := file.Write(entry)

	if writeErr == nil && written != len(entry) {
		writeErr = io.ErrShortWrite
	}

	if writeErr == nil {
		writeErr = file.Sync()
	}

	closeErr := file.Close()

	if writeErr != nil {
		return fmt.Errorf(
			"failed to persist audit epoch: %w",
			writeErr,
		)
	}

	if closeErr != nil {
		return closeErr
	}

	return nil
}
