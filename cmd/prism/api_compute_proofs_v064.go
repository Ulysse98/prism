package main

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"prism/internal/blockchain"
	"prism/internal/compute"
	"prism/internal/p2p"
	"prism/internal/usefulwork"
)

// apiComputeProofSnapshotV064 exposes only public, verified job/proof data.
// It is not a settlement authorization and is not a proof of network finality.
type apiComputeProofSnapshotV064 struct {
	Job         compute.Job      `json:"job"`
	Proof       usefulwork.Proof `json:"proof"`
	Block       uint64           `json:"block"`
	BlockHash   string           `json:"blockHash"`
	ChainID     string           `json:"chainId"`
	GenesisHash string           `json:"genesisHash"`
}

// computeProofSnapshotV064 requires the caller to supply an independently
// validated chain. The public handler obtains it through api.loadState().
func computeProofSnapshotV064(job compute.Job, chain *blockchain.Blockchain) (apiComputeProofSnapshotV064, error) {
	if job.Status != compute.JobStatusVerified || job.ProofID == "" {
		return apiComputeProofSnapshotV064{}, fmt.Errorf("compute job has no verified proof")
	}
	if job.ID == "" || job.ID != compute.CalculateJobID(job) {
		return apiComputeProofSnapshotV064{}, fmt.Errorf("compute job ID mismatch")
	}
	if chain == nil || len(chain.Blocks) == 0 || chain.Blocks[0].Hash == "" {
		return apiComputeProofSnapshotV064{}, fmt.Errorf("missing genesis block")
	}
	genesisHash := chain.Blocks[0].Hash
	chainID := p2p.MakeChainID(genesisHash)

	var selected *usefulwork.Proof
	var foundBlock uint64
	var foundBlockHash string
	for _, block := range chain.Blocks {
		for _, candidate := range block.UsefulWork {
			if candidate.ID != job.ProofID {
				continue
			}
			if selected != nil {
				return apiComputeProofSnapshotV064{}, fmt.Errorf("duplicate proof ID on chain")
			}
			copyProof := candidate
			selected = &copyProof
			foundBlock = block.Height
			foundBlockHash = block.Hash
		}
	}
	if selected == nil || foundBlockHash == "" || foundBlock == 0 {
		return apiComputeProofSnapshotV064{}, fmt.Errorf("verified compute proof not found in a non-genesis block")
	}
	proof := *selected
	if proof.ProofVersion != usefulwork.ComputeProofVersion ||
		proof.Task.Type != usefulwork.TaskTypeMLInferenceQuantized {
		return apiComputeProofSnapshotV064{}, fmt.Errorf("v0.64 requires quantized ML Proof v2")
	}
	if proof.Worker != job.Worker || proof.Task.ID != job.Task.ID ||
		!reflect.DeepEqual(proof.Task, job.Task) {
		return apiComputeProofSnapshotV064{}, fmt.Errorf("on-chain proof does not match marketplace job")
	}
	if err := usefulwork.VerifyProof(proof); err != nil {
		return apiComputeProofSnapshotV064{}, fmt.Errorf("invalid on-chain proof: %w", err)
	}
	if err := usefulwork.VerifyComputeProofContext(proof, job.ID, chainID, genesisHash); err != nil {
		return apiComputeProofSnapshotV064{}, fmt.Errorf("wrong on-chain proof context: %w", err)
	}
	units, err := usefulwork.WorkUnits(proof.Task)
	if err != nil || units != job.WorkUnits || units != proof.Score {
		return apiComputeProofSnapshotV064{}, fmt.Errorf("inconsistent compute work units")
	}
	return apiComputeProofSnapshotV064{
		Job: job, Proof: proof, Block: foundBlock, BlockHash: foundBlockHash,
		ChainID: chainID, GenesisHash: genesisHash,
	}, nil
}

// GET /api/v1/compute/proofs/{jobId} is read-only and only returns PoUW
// Proof v2 after a successful on-chain settlement on the local node.
func (api *apiServer) handleComputeProofV064(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		apiWriteError(writer, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	jobID := strings.TrimPrefix(request.URL.Path, "/api/v1/compute/proofs/")
	if jobID == "" || strings.Contains(jobID, "/") || strings.TrimSpace(jobID) != jobID {
		apiWriteError(writer, http.StatusBadRequest, fmt.Errorf("invalid compute proof job ID"))
		return
	}
	if api == nil || api.computeMarket == nil {
		apiWriteError(writer, http.StatusServiceUnavailable, fmt.Errorf("compute marketplace unavailable"))
		return
	}

	// Settlement writes are serialized under stateMu. The snapshot must
	// cover the marketplace lookup and local chain read together.
	api.stateMu.Lock()
	defer api.stateMu.Unlock()

	job, err := api.computeMarket.Get(jobID)
	if err != nil {
		apiWriteError(writer, http.StatusNotFound, err)
		return
	}
	if job.Status != compute.JobStatusVerified || job.ProofID == "" {
		apiWriteError(writer, http.StatusConflict, fmt.Errorf("job not yet verified on-chain"))
		return
	}
	chain, _, _, err := api.loadState()
	if err != nil {
		apiWriteError(writer, http.StatusInternalServerError, fmt.Errorf("unable to read validated local chain: %w", err))
		return
	}
	snapshot, err := computeProofSnapshotV064(job, chain)
	if err != nil {
		apiWriteError(writer, http.StatusConflict, err)
		return
	}
	apiWriteJSON(writer, http.StatusOK, snapshot)
}
