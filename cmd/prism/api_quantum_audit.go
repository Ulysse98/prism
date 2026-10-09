package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"prism/internal/compute"
	"prism/internal/p2p"
	"prism/internal/usefulwork"
)

func loadQuantumAuditPolicy(
	path string,
) (usefulwork.QuantumQuorumPolicy, error) {
	var policy usefulwork.QuantumQuorumPolicy

	info, err := os.Stat(path)
	if err != nil {
		return policy, err
	}

	if !info.Mode().IsRegular() ||
		info.Size() == 0 ||
		info.Size() > 16384 {
		return policy, fmt.Errorf("invalid quantum policy file")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return policy, err
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&policy); err != nil {
		return policy, err
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return policy, fmt.Errorf("unexpected trailing policy data")
	}

	if policy.Model != "bell-ideal-v1" {
		return policy, fmt.Errorf("unsupported quantum model")
	}

	count := len(policy.AuthorizedVerifiers)
	if count == 0 || count > 64 ||
		policy.RequiredApprovals < 1 ||
		policy.RequiredApprovals > count {
		return policy, fmt.Errorf("invalid quorum policy")
	}

	seen := make(map[string]bool, count)

	for _, address := range policy.AuthorizedVerifiers {
		if !strings.HasPrefix(address, "prism_") ||
			len(address) != 46 ||
			strings.ToLower(address) != address {
			return policy, fmt.Errorf("invalid verifier address")
		}

		decoded, err := hex.DecodeString(address[6:])
		if err != nil || len(decoded) != 20 {
			return policy, fmt.Errorf("invalid verifier encoding")
		}

		if seen[address] {
			return policy, fmt.Errorf("duplicate verifier")
		}

		seen[address] = true
	}

	return policy, nil
}

func (api *apiServer) handleQuantumAudit(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		apiWriteError(
			writer,
			http.StatusMethodNotAllowed,
			fmt.Errorf("method not allowed"),
		)
		return
	}

	if api == nil || api.computeMarket == nil ||
		api.quantumAuditPolicy == nil {
		apiWriteError(
			writer,
			http.StatusServiceUnavailable,
			fmt.Errorf("quantum audit unavailable"),
		)
		return
	}

	if api.quantumAuditTokenHash == nil {
		apiWriteError(
			writer,
			http.StatusServiceUnavailable,
			fmt.Errorf("quantum audit authentication unavailable"),
		)
		return
	}

	if !verifyQuantumAuditBearer(
		request,
		api.quantumAuditTokenHash,
	) {
		writer.Header().Set(
			"WWW-Authenticate",
			`Bearer realm="prism-quantum-audit"`,
		)

		apiWriteError(
			writer,
			http.StatusUnauthorized,
			fmt.Errorf("quantum audit authorization required"),
		)
		return
	}
	if !api.quantumAuditBusy.CompareAndSwap(false, true) {
		writer.Header().Set("Retry-After", "1")
		apiWriteError(
			writer,
			http.StatusTooManyRequests,
			fmt.Errorf("quantum audit already running"),
		)
		return
	}

	defer api.quantumAuditBusy.Store(false)

	allowed, retry := api.quantumAuditRate.allow(time.Now())
	if !allowed {
		seconds := int64((retry + time.Second - 1) / time.Second)
		if seconds < 1 {
			seconds = 1
		}

		writer.Header().Set("Retry-After", fmt.Sprint(seconds))
		apiWriteError(
			writer,
			http.StatusTooManyRequests,
			fmt.Errorf("quantum audit rate limit exceeded"),
		)
		return
	}

	jobID := request.PathValue("id")
	if strings.TrimSpace(jobID) == "" {
		apiWriteError(
			writer,
			http.StatusBadRequest,
			fmt.Errorf("missing job ID"),
		)
		return
	}

	request.Body = http.MaxBytesReader(
		writer,
		request.Body,
		65536,
	)

	var payload struct {
		Reports []usefulwork.QuantumVerificationReport `json:"reports"`
	}

	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&payload); err != nil {
		apiWriteError(writer, http.StatusBadRequest, err)
		return
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		apiWriteError(
			writer,
			http.StatusBadRequest,
			fmt.Errorf("unexpected trailing request data"),
		)
		return
	}

	policy := *api.quantumAuditPolicy

	if len(payload.Reports) > len(policy.AuthorizedVerifiers) {
		apiWriteError(
			writer,
			http.StatusBadRequest,
			fmt.Errorf("too many reports"),
		)
		return
	}

	api.stateMu.Lock()

	job, err := api.computeMarket.Get(jobID)
	if err != nil {
		api.stateMu.Unlock()
		apiWriteError(writer, http.StatusNotFound, err)
		return
	}

	if job.Status != compute.JobStatusVerified ||
		job.ProofID == "" ||
		job.Task.Type != usefulwork.TaskTypeQuantumSimulation {
		api.stateMu.Unlock()
		apiWriteError(
			writer,
			http.StatusConflict,
			fmt.Errorf("job is not a verified quantum job"),
		)
		return
	}

	chain, _, _, err := api.loadState()
	api.stateMu.Unlock()
	if err != nil {
		apiWriteError(writer, http.StatusInternalServerError, err)
		return
	}

	if len(chain.Blocks) == 0 || chain.Blocks[0].Hash == "" {
		apiWriteError(
			writer,
			http.StatusConflict,
			fmt.Errorf("missing genesis block"),
		)
		return
	}

	genesisHash := chain.Blocks[0].Hash
	chainID := p2p.MakeChainID(genesisHash)

	var proof usefulwork.Proof
	found := false

	for _, block := range chain.Blocks {
		for _, candidate := range block.UsefulWork {
			if candidate.ID == job.ProofID {
				proof = candidate
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		apiWriteError(
			writer,
			http.StatusConflict,
			fmt.Errorf("quantum proof missing from chain"),
		)
		return
	}

	if proof.Worker != job.Worker ||
		proof.Task.ID != job.Task.ID {
		apiWriteError(
			writer,
			http.StatusConflict,
			fmt.Errorf("proof does not match marketplace job"),
		)
		return
	}

	if err := usefulwork.VerifyProof(proof); err != nil {
		apiWriteError(writer, http.StatusConflict, err)
		return
	}

	if err := usefulwork.VerifyComputeProofContext(
		proof,
		job.ID,
		chainID,
		genesisHash,
	); err != nil {
		apiWriteError(writer, http.StatusConflict, err)
		return
	}

	audit, auditErr := usefulwork.AuditQuantumVerification(
		proof,
		payload.Reports,
		policy,
	)

	if auditErr != nil && audit.Reason == "" {
		apiWriteError(
			writer,
			http.StatusInternalServerError,
			auditErr,
		)
		return
	}

	apiWriteJSON(writer, http.StatusOK, map[string]any{
		"audit":         audit,
		"observational": true,
	})
}
