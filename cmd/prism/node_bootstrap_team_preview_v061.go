package main

import (
	"flag"
	"fmt"
	"math"
	"strings"
	"time"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/reserved"
	"prism/internal/storage"
)

// node-bootstrap-team-preview is deliberately READ ONLY. A separate,
// coordinated production path is required before any network mutation.
// Signing uses local validator keys but does not print or export them.
func runNodeBootstrapTeamPreviewV061(args []string) error {
	flags := flag.NewFlagSet("node-bootstrap-team-preview", flag.ContinueOnError)
	dataDir := flags.String("data", "", "existing node data directory")
	names := flags.String("authorities", "", "comma-separated local authority wallet names")
	signerNames := flags.String("signers", "", "comma-separated local validator wallet names")
	expectedTip := flags.String("expected-tip", "", "expected canonical tip hash at height 5")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	if *dataDir == "" || *names == "" || *signerNames == "" || *expectedTip == "" {
		return fmt.Errorf("required: -data, -authorities, -signers, -expected-tip")
	}
	authorityLabels, err := parseBootstrapNamesV061(*names)
	if err != nil {
		return fmt.Errorf("authorities: %w", err)
	}
	if len(authorityLabels) != 2 {
		return fmt.Errorf("v0.61 preview requires exactly 2 Team authorities (2-of-2)")
	}
	signerLabels, err := parseBootstrapNamesV061(*signerNames)
	if err != nil {
		return fmt.Errorf("signers: %w", err)
	}
	if !storage.Exists(*dataDir) {
		return fmt.Errorf("refusing missing node data directory; will not create another chain")
	}
	chain, pos, wallets, err := storage.Load(*dataDir)
	if err != nil {
		return fmt.Errorf("load canonical chain: %w", err)
	}
	if len(chain.Blocks) != 6 {
		return fmt.Errorf("v0.61 preview requires existing chain exactly at height 5")
	}
	genesis := chain.Blocks[0].Hash
	last := chain.Blocks[len(chain.Blocks)-1]
	if last.Height != 5 || last.Hash != *expectedTip ||
		last.Hash != "d5f0ae89725004f3d99f66a5d8b5b2d5061b941980c3e59c0751ca01cd0da521" ||
		genesis != "807567a62284fb7990ab6a3d75c0a8255928a5d3b8681ec7fb86eac8615c5f85" {
		return fmt.Errorf("chain tip/genesis mismatch; refusing to sign")
	}
	chainID, err := chain.ChainID()
	if err != nil {
		return err
	}
	if chainID != "prism-ceea53aa87bcae01" {
		return fmt.Errorf("unsupported chain ID: %s", chainID)
	}

	authorityAddresses := make([]string, 0, len(authorityLabels))
	for _, label := range authorityLabels {
		w, ok := wallets[label]
		if !ok || w == nil {
			return fmt.Errorf("authority wallet %q not found locally", label)
		}
		authorityAddresses = append(authorityAddresses, w.Address)
	}
	bootstrap, err := reserved.NewGovernanceBootstrap(
		chainID, genesis, consensus.ReservedPoolTeam, authorityAddresses, 2, 1, 6,
	)
	if err != nil {
		return fmt.Errorf("bootstrap intent: %w", err)
	}
	for _, label := range signerLabels {
		w, ok := wallets[label]
		if !ok || w == nil {
			return fmt.Errorf("signer wallet %q not found locally", label)
		}
		if err := bootstrap.AddApproval(w); err != nil {
			return fmt.Errorf("signer %q failed: %w", label, err)
		}
	}
	if err := chain.ValidateBootstrapCandidateV061(bootstrap, pos); err != nil {
		return fmt.Errorf("canonical bootstrap preflight denied: %w", err)
	}

	proposer, err := pos.SelectProposer(last.Hash, 6)
	if err != nil {
		return fmt.Errorf("select proposer: %w", err)
	}
	policy := consensus.DefaultRewardPolicy()
	supply := consensus.DefaultSupplyPolicy()
	if err := policy.Validate(); err != nil {
		return err
	}
	if err := supply.Validate(); err != nil {
		return err
	}
	emission, err := chain.GetEmissionState()
	if err != nil {
		return fmt.Errorf("emission replay: %w", err)
	}
	reward, err := consensus.BoundedPoolReward(
		policy.ProposerReward, emission.ProposerEmission, supply.ProposerRewardPool,
	)
	if err != nil {
		return fmt.Errorf("bootstrap reward: %w", err)
	}
	if last.Height == math.MaxUint64 {
		return fmt.Errorf("height overflow")
	}
	block := blockchain.Block{
		Height:               6,
		Timestamp:            time.Now().UTC(),
		PreviousHash:         last.Hash,
		Proposer:             proposer.Address,
		Reward:               reward,
		GovernanceBootstraps: []reserved.GovernanceBootstrap{bootstrap},
	}
	block.Hash = blockchain.CalculateHash(block)

	// Validate the full consensus path on an IN-MEMORY clone, never on the
	// canonical chain. No storage.Save or network broadcast occurs here.
	snapshot := &blockchain.Blockchain{
		Blocks:       append([]blockchain.Block(nil), chain.Blocks...),
		LockedStakes: make(map[string]uint64, len(chain.LockedStakes)),
		Config:       chain.Config,
	}
	for address, stake := range chain.LockedStakes {
		snapshot.LockedStakes[address] = stake
	}
	if err := snapshot.AppendValidatedBlock(block, pos); err != nil {
		return fmt.Errorf("full candidate consensus rejected: %w", err)
	}
	state, err := snapshot.GetGovernanceState()
	if err != nil {
		return fmt.Errorf("replay accepted block but governance failed: %w", err)
	}
	if len(state.CurrentPolicy.Team) != 2 || state.CurrentPolicy.TeamThreshold != 2 {
		return fmt.Errorf("governance replay did not activate expected 2-of-2 Team policy")
	}
	if !snapshot.ValidateChain(pos) {
		return fmt.Errorf("appended in-memory candidate fails ValidateChain")
	}
	fmt.Println("=== V0.61 SIGNED CANONICAL BOOTSTRAP PREVIEW PASSED ===")
	fmt.Println("Chain ID:", chainID)
	fmt.Println("Height:", block.Height)
	fmt.Println("Proposer:", block.Proposer)
	fmt.Println("Bootstrap ID:", bootstrap.ID)
	fmt.Println("Candidate block hash:", block.Hash)
	fmt.Println("Signer count:", len(bootstrap.Approvals))
	fmt.Println("Authority threshold: 2 of 2")
	fmt.Println("Canonical disk chain NOT MODIFIED. No block was broadcast.")
	return nil
}

func parseBootstrapNamesV061(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, fmt.Errorf("empty wallet list")
	}
	names := strings.Split(value, ",")
	seen := make(map[string]bool, len(names))
	for i, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			return nil, fmt.Errorf("blank or duplicate wallet label")
		}
		names[i] = name
		seen[name] = true
	}
	return names, nil
}
