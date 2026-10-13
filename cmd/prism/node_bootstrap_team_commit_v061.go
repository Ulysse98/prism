package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/reserved"
	"prism/internal/storage"
)

const (
	bootstrapV061Genesis = "807567a62284fb7990ab6a3d75c0a8255928a5d3b8681ec7fb86eac8615c5f85"
	bootstrapV061Tip5    = "d5f0ae89725004f3d99f66a5d8b5b2d5061b941980c3e59c0751ca01cd0da521"
	bootstrapV061ChainID = "prism-ceea53aa87bcae01"
	bootstrapV061Confirm = "COMMIT-PRISM-BOOTSTRAP-BLOCK-6"
)

// node-bootstrap-team-commit is an intentionally explicit, local-disk
// production path. It never broadcasts and MUST run only after coordinated
// validator upgrade and shutdown of all writers to this data directory.
// It acquires the SAME directory lock used by the Prism HTTP API.
// The CLI writes ONLY chain.json; wallets.json is loaded, never rewritten.
func runNodeBootstrapTeamCommitV061(args []string) error {
	flags := flag.NewFlagSet("node-bootstrap-team-commit", flag.ContinueOnError)
	dataDir := flags.String("data", "", "existing canonical node directory")
	authorities := flags.String("authorities", "", "two local Team authority names")
	signers := flags.String("signers", "", "local validator names giving >2/3 signed stake")
	expectedTip := flags.String("expected-tip", "", "canonical block 5 tip hash")
	expectedID := flags.String("expected-bootstrap-id", "", "intent ID validated in preview")
	confirmation := flags.String("confirm-commit", "", "explicit confirmation phrase")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	if *confirmation != bootstrapV061Confirm {
		return fmt.Errorf("missing explicit -confirm-commit %s", bootstrapV061Confirm)
	}
	if *dataDir == "" || *authorities == "" || *signers == "" || *expectedTip == "" || *expectedID == "" {
		return fmt.Errorf("required: -data -authorities -signers -expected-tip -expected-bootstrap-id -confirm-commit")
	}
	if *expectedTip != bootstrapV061Tip5 {
		return fmt.Errorf("unrecognized checkpoint tip")
	}
	if !storage.Exists(*dataDir) {
		return fmt.Errorf("missing existing canonical chain; refusing to create network")
	}

	// Prevent writes while the API is live. A prior crash may leave this lock;
	// never delete the lock automatically and never force through it.
	absDir, err := filepath.Abs(*dataDir)
	if err != nil {
		return err
	}
	lock, err := acquirePrismAPIRuntimeLock(absDir)
	if err != nil {
		return fmt.Errorf("exclusive data lock unavailable (stop API 8081 and other writers first): %w", err)
	}
	defer func() {
		if err := lock.Release(); err != nil {
			fmt.Fprintln(os.Stderr, "WARNING: unable to release node data lock:", err)
		}
	}()

	chain, pos, wallets, err := storage.Load(absDir)
	if err != nil {
		return fmt.Errorf("load canonical chain: %w", err)
	}
	if len(chain.Blocks) != 6 {
		return fmt.Errorf("expected exactly six stored blocks (heights 0..5)")
	}
	last := chain.Blocks[len(chain.Blocks)-1]
	if last.Height != 5 || last.Hash != *expectedTip || last.Hash != bootstrapV061Tip5 || chain.Blocks[0].Hash != bootstrapV061Genesis {
		return fmt.Errorf("checkpoint identity mismatch; no block written")
	}
	chainID, err := chain.ChainID()
	if err != nil {
		return err
	}
	if chainID != bootstrapV061ChainID {
		return fmt.Errorf("wrong chain ID %s", chainID)
	}
	if !chain.ValidateChain(pos) {
		return fmt.Errorf("canonical chain invalid")
	}
	authorityNames, err := parseBootstrapNamesV061(*authorities)
	if err != nil {
		return err
	}
	if len(authorityNames) != 2 {
		return fmt.Errorf("expected two Team authorities")
	}
	signerNames, err := parseBootstrapNamesV061(*signers)
	if err != nil {
		return err
	}
	authorityAddresses := make([]string, 0, 2)
	for _, name := range authorityNames {
		wallet := wallets[name]
		if wallet == nil {
			return fmt.Errorf("authority wallet %q absent", name)
		}
		authorityAddresses = append(authorityAddresses, wallet.Address)
	}
	bootstrap, err := reserved.NewGovernanceBootstrap(
		chainID, bootstrapV061Genesis, consensus.ReservedPoolTeam,
		authorityAddresses, 2, 1, 6,
	)
	if err != nil {
		return err
	}
	if bootstrap.ID != *expectedID {
		return fmt.Errorf("bootstrap intent differs from signed preview; expected %s got %s", *expectedID, bootstrap.ID)
	}
	for _, name := range signerNames {
		wallet := wallets[name]
		if wallet == nil {
			return fmt.Errorf("validator signer wallet %q absent", name)
		}
		if err := bootstrap.AddApproval(wallet); err != nil {
			return fmt.Errorf("signer %q: %w", name, err)
		}
	}
	if err := chain.ValidateBootstrapCandidateV061(bootstrap, pos); err != nil {
		return fmt.Errorf("bootstrap candidate refused: %w", err)
	}
	if last.Height == math.MaxUint64 {
		return fmt.Errorf("height overflow")
	}
	proposer, err := pos.SelectProposer(last.Hash, 6)
	if err != nil {
		return err
	}
	rewardPolicy := consensus.DefaultRewardPolicy()
	supplyPolicy := consensus.DefaultSupplyPolicy()
	if err := rewardPolicy.Validate(); err != nil {
		return err
	}
	if err := supplyPolicy.Validate(); err != nil {
		return err
	}
	emission, err := chain.GetEmissionState()
	if err != nil {
		return err
	}
	reward, err := consensus.BoundedPoolReward(
		rewardPolicy.ProposerReward, emission.ProposerEmission, supplyPolicy.ProposerRewardPool,
	)
	if err != nil {
		return err
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

	candidate := &blockchain.Blockchain{
		Blocks:       append([]blockchain.Block(nil), chain.Blocks...),
		LockedStakes: make(map[string]uint64, len(chain.LockedStakes)),
		Config:       chain.Config,
	}
	for key, value := range chain.LockedStakes {
		candidate.LockedStakes[key] = value
	}
	if err := candidate.AppendValidatedBlock(block, pos); err != nil {
		return fmt.Errorf("canonical consensus rejected bootstrap: %w", err)
	}
	if !candidate.ValidateChain(pos) {
		return fmt.Errorf("candidate chain invalid")
	}
	governance, err := candidate.GetGovernanceState()
	if err != nil {
		return fmt.Errorf("governance replay failed: %w", err)
	}
	if len(governance.CurrentPolicy.Team) != 2 || governance.CurrentPolicy.TeamThreshold != 2 {
		return fmt.Errorf("replayed Team authority policy unexpected")
	}
	want := append([]string(nil), authorityAddresses...)
	sort.Strings(want)
	for i := range want {
		if governance.CurrentPolicy.Team[i] != want[i] {
			return fmt.Errorf("replayed authority set differs from signed intent")
		}
	}

	// Persist only after all consensus checks, holding exclusive API lock.
	// Back up original chain.json and write a new temporary file on same disk.
	if err := commitBootstrapChainV061(absDir, candidate, pos, bootstrapV061Tip5); err != nil {
		return fmt.Errorf("unable to persist candidate: %w", err)
	}
	reloaded, rereadPoS, _, err := storage.Load(absDir)
	if err != nil {
		return fmt.Errorf("post-commit reload failed (check backup): %w", err)
	}
	if !reloaded.ValidateChain(rereadPoS) || len(reloaded.Blocks) != 7 ||
		reloaded.Blocks[6].Hash != block.Hash ||
		len(reloaded.Blocks[6].GovernanceBootstraps) != 1 ||
		reloaded.Blocks[6].GovernanceBootstraps[0].ID != bootstrap.ID {
		return fmt.Errorf("post-commit verification mismatch; stop and inspect chain backup")
	}
	state, err := reloaded.GetGovernanceState()
	if err != nil || len(state.CurrentPolicy.Team) != 2 || state.CurrentPolicy.TeamThreshold != 2 {
		return fmt.Errorf("post-commit Team governance verification failed; stop and inspect chain backup")
	}
	fmt.Println("=== V0.61 CANONICAL BLOCK 6 PERSISTED ===")
	fmt.Println("Chain ID:", chainID)
	fmt.Println("Height:", block.Height)
	fmt.Println("Block hash:", block.Hash)
	fmt.Println("Bootstrap ID:", bootstrap.ID)
	fmt.Println("Proposer:", block.Proposer)
	fmt.Println("Team policy: 2-of-2")
	fmt.Println("Data: chain.json updated; wallets.json untouched; NO P2P broadcast")
	return nil
}

func commitBootstrapChainV061(dataDir string, candidate *blockchain.Blockchain, pos *consensus.ProofOfStake, expectedPreviousTip string) error {
	if candidate == nil || pos == nil || !candidate.ValidateChain(pos) {
		return fmt.Errorf("invalid candidate")
	}
	target := filepath.Join(dataDir, "chain.json")
	original, err := os.ReadFile(target)
	if err != nil {
		return err
	}
	var old storage.ChainFile
	if err := json.Unmarshal(original, &old); err != nil {
		return err
	}
	if old.Blockchain == nil || len(old.Blockchain.Blocks) != 6 || old.Blockchain.Blocks[5].Hash != expectedPreviousTip {
		return fmt.Errorf("disk chain changed before persistence")
	}
	serialized, err := json.MarshalIndent(storage.ChainFile{Blockchain: candidate, Validators: pos.Validators}, "", "  ")
	if err != nil {
		return err
	}
	// Timestamped exclusive backup; an existing backup must never be overwritten.
	backupPath := filepath.Join(dataDir, "chain.json.v061-backup-"+time.Now().UTC().Format("20060102T150405.000000000Z"))
	backup, err := os.OpenFile(backupPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := backup.Write(original)
	if writeErr == nil {
		writeErr = backup.Sync()
	}
	closeErr := backup.Close()
	if writeErr != nil {
		return fmt.Errorf("backup failed: %w", writeErr)
	}
	if closeErr != nil {
		return closeErr
	}
	fmt.Println("Canonical chain backup:", backupPath)
	fmt.Printf("Original chain.json SHA-256: %x\n", sha256.Sum256(original))
	tmp, err := os.CreateTemp(dataDir, ".v061-chain-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(serialized); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	// Protect against another, non-cooperating writer changing chain.json.
	nowRaw, err := os.ReadFile(target)
	if err != nil {
		return err
	}
	if !bytes.Equal(nowRaw, original) {
		return fmt.Errorf("chain.json concurrently modified; refusing replacement")
	}
	if err := os.Rename(tmp.Name(), target); err != nil {
		return fmt.Errorf("atomic chain.json replacement failed; original and backup preserved: %w", err)
	}
	return nil
}
