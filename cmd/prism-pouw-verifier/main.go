// prism-pouw-verifier is an experimental, read-only local verifier.
// It does not authorize settlement or modify blockchain state.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"time"

	"prism/internal/blockchain"
	"prism/internal/compute"
	"prism/internal/p2p"
	"prism/internal/storage"
	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

const maxHTTPBodyV064 = 2 << 20

type proofSnapshotV064 struct {
	Job         compute.Job      `json:"job"`
	Proof       usefulwork.Proof `json:"proof"`
	Block       uint64           `json:"block"`
	BlockHash   string           `json:"blockHash"`
	ChainID     string           `json:"chainId"`
	GenesisHash string           `json:"genesisHash"`
}

type jobListV064 struct {
	Count int           `json:"count"`
	Jobs  []compute.Job `json:"jobs"`
}

type nodeStatusV064 struct {
	ChainValid  bool   `json:"chainValid"`
	ChainID     string `json:"chainId"`
	GenesisHash string `json:"genesisHash"`
}

func validateLocalAPIBaseV064(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	host := strings.ToLower(parsed.Hostname())
	if parsed.Scheme != "http" || (host != "127.0.0.1" && host != "localhost" && host != "::1") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "/api/v1" || parsed.Opaque != "" || parsed.RawPath != "" {
		return "", fmt.Errorf("v0.64 verifier requires an HTTP loopback API ending in /api/v1")
	}
	if parsed.Port() == "" {
		return "", fmt.Errorf("API port must be explicit")
	}
	return parsed.String(), nil
}

func readJSONV064(ctx context.Context, client *http.Client, endpoint string, dest any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: HTTP %d", request.URL.Path, response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxHTTPBodyV064+1))
	if err != nil {
		return err
	}
	if len(body) > maxHTTPBodyV064 {
		return fmt.Errorf("HTTP response too large")
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	if err := dec.Decode(dest); err != nil {
		return err
	}
	var remainder any
	if err := dec.Decode(&remainder); !errors.Is(err, io.EOF) {
		return fmt.Errorf("trailing or oversized JSON: %v", err)
	}
	return nil
}

// This validation uses the independently loaded, fully validated local chain.
// Neither the HTTP status nor an API-supplied block hash is finality evidence.
func validateProofSnapshotV064(job compute.Job, snap proofSnapshotV064, chain *blockchain.Blockchain) error {
	if chain == nil || len(chain.Blocks) == 0 || chain.Blocks[0].Hash == "" {
		return fmt.Errorf("missing local genesis")
	}
	genesis := chain.Blocks[0].Hash
	chainID := p2p.MakeChainID(genesis)
	if !reflect.DeepEqual(job, snap.Job) || job.ID == "" || job.ID != compute.CalculateJobID(job) || job.Status != compute.JobStatusVerified || job.ProofID == "" || job.Worker == "" {
		return fmt.Errorf("snapshot job does not match verified marketplace job")
	}
	proof := snap.Proof
	if snap.GenesisHash != genesis || snap.ChainID != chainID || snap.Block == 0 || snap.BlockHash == "" {
		return fmt.Errorf("snapshot chain identity mismatch")
	}
	if proof.ProofVersion != usefulwork.ComputeProofVersion || proof.Task.Type != usefulwork.TaskTypeMLInferenceQuantized || proof.ID != job.ProofID || proof.Worker != job.Worker || proof.Task.ID != job.Task.ID || !reflect.DeepEqual(proof.Task, job.Task) {
		return fmt.Errorf("snapshot proof is not the claimed quantized ML V2 result")
	}
	if err := usefulwork.VerifyComputeProofContext(proof, job.ID, chainID, genesis); err != nil {
		return err
	}
	// Full deterministic CPU recomputation and Ed25519 signature checking.
	if err := usefulwork.VerifyProof(proof); err != nil {
		return err
	}
	units, err := usefulwork.WorkUnits(proof.Task)
	if err != nil {
		return err
	}
	if units != proof.Score || units != job.WorkUnits {
		return fmt.Errorf("work units mismatch")
	}
	matches := 0
	for _, block := range chain.Blocks {
		for _, candidate := range block.UsefulWork {
			if candidate.ID != proof.ID {
				continue
			}
			matches++
			if block.Height != snap.Block || block.Hash != snap.BlockHash || !reflect.DeepEqual(candidate, proof) {
				return fmt.Errorf("proof snapshot differs from locally validated chain")
			}
		}
	}
	if matches != 1 {
		return fmt.Errorf("expected exactly one matching on-chain proof, got %d", matches)
	}
	return nil
}

func collectReportsV064(ctx context.Context, client *http.Client, apiBase string, chain *blockchain.Blockchain, verifier *wallet.Wallet, limit int) ([]usefulwork.ComputeVerificationReportV064, error) {
	if chain == nil || len(chain.Blocks) == 0 || verifier == nil {
		return nil, fmt.Errorf("missing local chain or verifier")
	}
	genesis := chain.Blocks[0].Hash
	chainID := p2p.MakeChainID(genesis)
	var status nodeStatusV064
	if err := readJSONV064(ctx, client, apiBase+"/status", &status); err != nil {
		return nil, fmt.Errorf("node status: %w", err)
	}
	if !status.ChainValid || status.ChainID != chainID || status.GenesisHash != genesis {
		return nil, fmt.Errorf("remote status conflicts with validated local chain")
	}
	values := url.Values{}
	values.Set("status", string(compute.JobStatusVerified))
	values.Set("task", usefulwork.TaskTypeMLInferenceQuantized)
	values.Set("limit", fmt.Sprint(limit))
	var listing jobListV064
	if err := readJSONV064(ctx, client, apiBase+"/compute/jobs?"+values.Encode(), &listing); err != nil {
		return nil, fmt.Errorf("marketplace discovery: %w", err)
	}
	if listing.Count != len(listing.Jobs) || len(listing.Jobs) > limit {
		return nil, fmt.Errorf("malformed marketplace listing")
	}
	reports := make([]usefulwork.ComputeVerificationReportV064, 0, len(listing.Jobs))
	seen := make(map[string]struct{})
	for _, job := range listing.Jobs {
		if _, duplicate := seen[job.ID]; duplicate {
			return nil, fmt.Errorf("duplicate marketplace job")
		}
		seen[job.ID] = struct{}{}
		if job.Status != compute.JobStatusVerified || job.Task.Type != usefulwork.TaskTypeMLInferenceQuantized || len(job.ID) != 64 {
			return nil, fmt.Errorf("unexpected marketplace job")
		}
		var snap proofSnapshotV064
		if err := readJSONV064(ctx, client, apiBase+"/compute/proofs/"+url.PathEscape(job.ID), &snap); err != nil {
			return nil, fmt.Errorf("proof lookup for %s: %w", job.ID, err)
		}
		if err := validateProofSnapshotV064(job, snap, chain); err != nil {
			return nil, fmt.Errorf("unverified snapshot %s: %w", job.ID, err)
		}
		report, err := usefulwork.SignComputeVerificationV064(snap.Proof, verifier)
		if err != nil {
			return nil, err
		}
		if err := usefulwork.VerifyComputeVerificationV064(report, snap.Proof); err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	return reports, nil
}

func run() error {
	flags := flag.NewFlagSet("prism-pouw-verifier", flag.ContinueOnError)
	api := flags.String("api", "http://127.0.0.1:8080/api/v1", "loopback Prism API")
	data := flags.String("data", "", "isolated testnet data directory containing chain and dev wallets")
	name := flags.String("verifier", "", "local testnet verifier wallet name (not worker)")
	poll := flags.Duration("poll", 0, "optional continuous scan interval (0 = one scan)")
	limit := flags.Int("limit", 100, "maximum VERIFIED ML jobs per scan (1..1000)")
	if err := flags.Parse(os.Args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 || *data == "" || strings.TrimSpace(*name) == "" || *limit < 1 || *limit > 1000 || *poll < 0 || (*poll > 0 && *poll < time.Second) {
		return fmt.Errorf("usage: prism-pouw-verifier -data ISOLATED_TESTNET -verifier NAME [-api http://127.0.0.1:8080/api/v1] [-poll 5s] [-limit 100]")
	}
	base, err := validateLocalAPIBaseV064(*api)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return fmt.Errorf("HTTP redirects are forbidden") }, Transport: &http.Transport{Proxy: nil}}
	seen := make(map[string]struct{})
	for {
		// Reload the validated chain on each cycle; avoid signing from stale state.
		chain, _, wallets, err := storage.Load(*data)
		if err != nil {
			return fmt.Errorf("local testnet state: %w", err)
		}
		verifier := wallets[*name]
		if verifier == nil {
			return fmt.Errorf("unknown local verifier wallet %q", *name)
		}
		reports, err := collectReportsV064(context.Background(), client, base, chain, verifier, *limit)
		if err != nil {
			return err
		}
		for _, report := range reports {
			if _, exists := seen[report.ID]; exists {
				continue
			}
			encoded, err := json.Marshal(report)
			if err != nil {
				return err
			}
			fmt.Println(string(encoded)) // JSON lines; output contains NO private keys.
			seen[report.ID] = struct{}{}
		}
		if *poll == 0 {
			return nil
		}
		time.Sleep(*poll)
	}
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "PRISM v0.64 verifier:", err)
		os.Exit(1)
	}
}
