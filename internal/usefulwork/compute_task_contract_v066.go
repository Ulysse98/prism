package usefulwork

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const (
	ComputeTaskContractVersionV066 uint8 = 1
	computeTaskContractDomainV066        = "Prism/PoUW/RealComputeTask/v0.66\x00"
)

// ComputeTaskContractV066 commits to a proposed job and its exact input and
// executable/model artifact. It is not a proof of execution, verifier
// attestation, consensus message or authorization to release escrow funds.
type ComputeTaskContractV066 struct {
	Version      uint8  `json:"version"`
	ChainID      string `json:"chain_id"`
	GenesisHash  string `json:"genesis_hash"`
	JobID        string `json:"job_id"`
	TaskID       string `json:"task_id"`
	InputHash    string `json:"input_hash"`
	WorkloadID   string `json:"workload_id"`
	ArtifactHash string `json:"artifact_hash"`
}

func validComputeIDV066(s string) bool {
	if len(s) == 0 || len(s) > 128 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_' ||
			c == '.' || c == ':' {
			continue
		}
		return false
	}
	return true
}

func validSHA256HexV066(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

func ValidateComputeTaskContractV066(c ComputeTaskContractV066) error {
	if c.Version != ComputeTaskContractVersionV066 {
		return fmt.Errorf("unsupported v0.66 compute task contract version")
	}
	for _, entry := range []struct{ name, value string }{
		{"chain_id", c.ChainID}, {"job_id", c.JobID},
		{"task_id", c.TaskID}, {"workload_id", c.WorkloadID},
	} {
		if !validComputeIDV066(entry.value) {
			return fmt.Errorf("invalid %s", entry.name)
		}
	}
	for _, entry := range []struct{ name, value string }{
		{"genesis_hash", c.GenesisHash}, {"input_hash", c.InputHash},
		{"artifact_hash", c.ArtifactHash},
	} {
		if !validSHA256HexV066(entry.value) {
			return fmt.Errorf("invalid %s: expected lowercase SHA-256 hex", entry.name)
		}
	}
	return nil
}

// ComputeTaskContractDigestV066 returns a domain-separated, fixed-order digest.
// It commits only to these declared fields, not to external artifact contents.
func ComputeTaskContractDigestV066(c ComputeTaskContractV066) ([32]byte, error) {
	if err := ValidateComputeTaskContractV066(c); err != nil {
		return [32]byte{}, err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return [32]byte{}, err
	}
	preimage := make([]byte, 0, len(computeTaskContractDomainV066)+len(data))
	preimage = append(preimage, computeTaskContractDomainV066...)
	preimage = append(preimage, data...)
	return sha256.Sum256(preimage), nil
}

func ComputeTaskContractIDV066(c ComputeTaskContractV066) (string, error) {
	digest, err := ComputeTaskContractDigestV066(c)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(digest[:]), nil
}
