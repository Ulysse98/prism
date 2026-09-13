package p2p

import (
	"encoding/json"
	"testing"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/reserved"
	"prism/internal/storage"
	"prism/internal/transaction"
	"prism/internal/wallet"
)

func TestReservedTransferProposalSurvivesWireAndStorage(
	t *testing.T,
) {
	validator, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	authority, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	recipient, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	sink, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	source, err := blockchain.NewBlockchain(
		map[string]uint64{
			validator.Address: 1000,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	const stake uint64 = 10

	if err := source.LockStake(
		validator.Address,
		stake,
	); err != nil {
		t.Fatal(err)
	}

	pos := consensus.NewProofOfStake()

	if err := pos.Register(
		validator.Address,
		stake,
	); err != nil {
		t.Fatal(err)
	}

	source.Config = blockchain.ChainConfig{
		ReservedAuthorities: reserved.AuthorityPolicy{
			Treasury: []string{
				authority.Address,
			},
			TreasuryThreshold: 1,
		},
	}

	appendFiller := func() {
		t.Helper()

		nonce, err := source.NonceOf(
			validator.Address,
		)
		if err != nil {
			t.Fatal(err)
		}

		tx := transaction.New(
			validator.Address,
			sink.Address,
			1,
			nonce,
			validator.PublicKeyHex(),
		)

		if err := tx.Sign(
			validator.PrivateKey,
		); err != nil {
			t.Fatal(err)
		}

		previous :=
			source.Blocks[len(source.Blocks)-1]

		proposer, err := pos.SelectProposer(
			previous.Hash,
			previous.Height+1,
		)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := source.AddBlock(
			[]transaction.Transaction{
				tx,
			},
			nil,
			proposer.Address,
			pos,
		); err != nil {
			t.Fatal(err)
		}
	}

	for {
		last :=
			source.Blocks[len(source.Blocks)-1]

		if last.Height >=
			blockchain.GovernedReservedTransferActivationHeight-1 {

			break
		}

		appendFiller()
	}

	receiverChain := &blockchain.Blockchain{
		Blocks: append(
			[]blockchain.Block(nil),
			source.Blocks...,
		),
		LockedStakes: map[string]uint64{
			validator.Address: stake,
		},
		Config: source.Config,
	}

	dataDir := t.TempDir()

	wallets := map[string]*wallet.Wallet{
		"Validator": validator,
		"Authority": authority,
		"Recipient": recipient,
		"Sink":      sink,
	}

	receiver := NewServer(
		"reserved-transfer-receiver",
		"127.0.0.1:0",
		dataDir,
		receiverChain,
		pos,
		wallets,
	)

	chainID, err := source.ChainID()
	if err != nil {
		t.Fatal(err)
	}

	proposal :=
		reserved.NewReservedTransferProposal(
			chainID,
			1,
			consensus.ReservedPoolTreasury,
			recipient.Address,
			100,
		)

	if err := proposal.AddApproval(
		authority.Address,
		authority.PublicKeyHex(),
		authority.PrivateKey,
	); err != nil {
		t.Fatal(err)
	}

	previous :=
		source.Blocks[len(source.Blocks)-1]

	proposer, err := pos.SelectProposer(
		previous.Hash,
		previous.Height+1,
	)
	if err != nil {
		t.Fatal(err)
	}

	block, err :=
		source.AddReservedTransferProposalBlock(
			[]reserved.ReservedTransferProposal{
				proposal,
			},
			proposer.Address,
			pos,
		)
	if err != nil {
		t.Fatal(err)
	}

	message := BlockMessage{
		Type:    MessageBlock,
		ChainID: chainID,
		Block:   block,
	}

	wire, err := json.Marshal(
		message,
	)
	if err != nil {
		t.Fatal(err)
	}

	var decoded BlockMessage

	if err := json.Unmarshal(
		wire,
		&decoded,
	); err != nil {
		t.Fatal(err)
	}

	if len(decoded.Block.ReservedTransferProposals) != 1 {
		t.Fatalf(
			"expected one transfer proposal on wire, got %d",
			len(decoded.Block.ReservedTransferProposals),
		)
	}

	wireProposal :=
		decoded.Block.ReservedTransferProposals[0]

	if wireProposal.ID != proposal.ID {
		t.Fatal(
			"wire encoding changed reserved transfer proposal ID",
		)
	}

	if len(wireProposal.Approvals) != 1 {
		t.Fatalf(
			"expected one transfer approval on wire, got %d",
			len(wireProposal.Approvals),
		)
	}

	if wireProposal.Approvals[0].Signature !=
		proposal.Approvals[0].Signature {

		t.Fatal(
			"wire encoding changed transfer approval signature",
		)
	}

	appended, err :=
		receiver.acceptBlock(
			decoded.Block,
		)
	if err != nil {
		t.Fatal(err)
	}

	if !appended {
		t.Fatal(
			"expected reserved transfer proposal block to append",
		)
	}

	loadedChain, loadedPoS, _, err :=
		storage.Load(
			dataDir,
		)
	if err != nil {
		t.Fatal(err)
	}

	if !loadedChain.ValidateChain(
		loadedPoS,
	) {
		t.Fatal(
			"reloaded transfer proposal chain failed validation",
		)
	}

	tip :=
		loadedChain.Blocks[len(loadedChain.Blocks)-1]

	if tip.Hash != block.Hash {
		t.Fatal(
			"storage reload changed proposal block hash",
		)
	}

	if len(tip.ReservedTransferProposals) != 1 {
		t.Fatalf(
			"storage reload lost transfer proposal: got %d",
			len(tip.ReservedTransferProposals),
		)
	}

	storedProposal :=
		tip.ReservedTransferProposals[0]

	if storedProposal.ID != proposal.ID {
		t.Fatal(
			"storage reload changed transfer proposal ID",
		)
	}

	if len(storedProposal.Approvals) != 1 ||
		storedProposal.Approvals[0].Signature !=
			proposal.Approvals[0].Signature {

		t.Fatal(
			"storage reload changed transfer proposal approval",
		)
	}

	governance, err :=
		loadedChain.GetGovernanceState()
	if err != nil {
		t.Fatal(err)
	}

	pending, exists :=
		governance.GetPendingReservedTransferProposal(
			proposal.ID,
		)

	if !exists {
		t.Fatal(
			"reloaded governance state lost pending transfer proposal",
		)
	}

	if pending.Proposal.ID != proposal.ID {
		t.Fatal(
			"reloaded pending transfer proposal ID mismatch",
		)
	}
}

func relayReservedTransferBlock(
	t *testing.T,
	receiver *Server,
	chainID string,
	block blockchain.Block,
) blockchain.Block {
	t.Helper()

	message := BlockMessage{
		Type:    MessageBlock,
		ChainID: chainID,
		Block:   block,
	}

	wire, err := json.Marshal(
		message,
	)
	if err != nil {
		t.Fatal(err)
	}

	var decoded BlockMessage

	if err := json.Unmarshal(
		wire,
		&decoded,
	); err != nil {
		t.Fatal(err)
	}

	appended, err :=
		receiver.acceptBlock(
			decoded.Block,
		)
	if err != nil {
		t.Fatal(err)
	}

	if !appended {
		t.Fatal(
			"expected relayed block to append",
		)
	}

	return decoded.Block
}

func TestReservedTransferExecutionSurvivesWireAndStorage(
	t *testing.T,
) {
	validator, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	authority, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	recipient, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	sink, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	source, err := blockchain.NewBlockchain(
		map[string]uint64{
			validator.Address: 1000,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	const stake uint64 = 10

	if err := source.LockStake(
		validator.Address,
		stake,
	); err != nil {
		t.Fatal(err)
	}

	pos := consensus.NewProofOfStake()

	if err := pos.Register(
		validator.Address,
		stake,
	); err != nil {
		t.Fatal(err)
	}

	source.Config = blockchain.ChainConfig{
		ReservedAuthorities: reserved.AuthorityPolicy{
			Treasury: []string{
				authority.Address,
			},
			TreasuryThreshold: 1,
		},
	}

	appendSourceFiller := func() blockchain.Block {
		t.Helper()

		nonce, err := source.NonceOf(
			validator.Address,
		)
		if err != nil {
			t.Fatal(err)
		}

		tx := transaction.New(
			validator.Address,
			sink.Address,
			1,
			nonce,
			validator.PublicKeyHex(),
		)

		if err := tx.Sign(
			validator.PrivateKey,
		); err != nil {
			t.Fatal(err)
		}

		previous :=
			source.Blocks[len(source.Blocks)-1]

		proposer, err := pos.SelectProposer(
			previous.Hash,
			previous.Height+1,
		)
		if err != nil {
			t.Fatal(err)
		}

		block, err := source.AddBlock(
			[]transaction.Transaction{
				tx,
			},
			nil,
			proposer.Address,
			pos,
		)
		if err != nil {
			t.Fatal(err)
		}

		return block
	}

	// Stop one block before governed-transfer activation.
	for {
		last :=
			source.Blocks[len(source.Blocks)-1]

		if last.Height >=
			blockchain.GovernedReservedTransferActivationHeight-1 {

			break
		}

		appendSourceFiller()
	}

	receiverChain := &blockchain.Blockchain{
		Blocks: append(
			[]blockchain.Block(nil),
			source.Blocks...,
		),
		LockedStakes: map[string]uint64{
			validator.Address: stake,
		},
		Config: source.Config,
	}

	dataDir := t.TempDir()

	wallets := map[string]*wallet.Wallet{
		"Validator": validator,
		"Authority": authority,
		"Recipient": recipient,
		"Sink":      sink,
	}

	receiver := NewServer(
		"reserved-transfer-execution-receiver",
		"127.0.0.1:0",
		dataDir,
		receiverChain,
		pos,
		wallets,
	)

	chainID, err := source.ChainID()
	if err != nil {
		t.Fatal(err)
	}

	proposal :=
		reserved.NewReservedTransferProposal(
			chainID,
			1,
			consensus.ReservedPoolTreasury,
			recipient.Address,
			100,
		)

	if err := proposal.AddApproval(
		authority.Address,
		authority.PublicKeyHex(),
		authority.PrivateKey,
	); err != nil {
		t.Fatal(err)
	}

	previous :=
		source.Blocks[len(source.Blocks)-1]

	proposer, err := pos.SelectProposer(
		previous.Hash,
		previous.Height+1,
	)
	if err != nil {
		t.Fatal(err)
	}

	proposalBlock, err :=
		source.AddReservedTransferProposalBlock(
			[]reserved.ReservedTransferProposal{
				proposal,
			},
			proposer.Address,
			pos,
		)
	if err != nil {
		t.Fatal(err)
	}

	decodedProposalBlock :=
		relayReservedTransferBlock(
			t,
			receiver,
			chainID,
			proposalBlock,
		)

	if len(
		decodedProposalBlock.ReservedTransferProposals,
	) != 1 {
		t.Fatal(
			"wire lost reserved transfer proposal",
		)
	}

	beforeExecution, err :=
		receiver.Chain.BalanceOf(
			recipient.Address,
		)
	if err != nil {
		t.Fatal(err)
	}

	governance, err :=
		source.GetGovernanceState()
	if err != nil {
		t.Fatal(err)
	}

	pending, exists :=
		governance.GetPendingReservedTransferProposal(
			proposal.ID,
		)

	if !exists {
		t.Fatal(
			"source lost pending transfer proposal",
		)
	}

	// Advance both peers until the next block is exactly the
	// consensus-derived execution boundary.
	for {
		last :=
			source.Blocks[len(source.Blocks)-1]

		if last.Height >=
			pending.ExecuteAfterHeight-1 {

			break
		}

		filler :=
			appendSourceFiller()

		relayReservedTransferBlock(
			t,
			receiver,
			chainID,
			filler,
		)
	}

	sourceTip :=
		source.Blocks[len(source.Blocks)-1]

	if sourceTip.Height+1 !=
		pending.ExecuteAfterHeight {

		t.Fatalf(
			"execution boundary mismatch: next=%d expected=%d",
			sourceTip.Height+1,
			pending.ExecuteAfterHeight,
		)
	}

	receiverTip :=
		receiver.Chain.Blocks[len(receiver.Chain.Blocks)-1]

	if receiverTip.Height != sourceTip.Height ||
		receiverTip.Hash != sourceTip.Hash {

		t.Fatal(
			"receiver did not converge before execution",
		)
	}

	execution :=
		reserved.NewReservedTransferExecution(
			proposal.ID,
		)

	proposer, err = pos.SelectProposer(
		sourceTip.Hash,
		sourceTip.Height+1,
	)
	if err != nil {
		t.Fatal(err)
	}

	executionBlock, err :=
		source.AddReservedTransferExecutionBlock(
			[]reserved.ReservedTransferExecution{
				execution,
			},
			proposer.Address,
			pos,
		)
	if err != nil {
		t.Fatal(err)
	}

	decodedExecutionBlock :=
		relayReservedTransferBlock(
			t,
			receiver,
			chainID,
			executionBlock,
		)

	if len(
		decodedExecutionBlock.ReservedTransferExecutions,
	) != 1 {
		t.Fatalf(
			"expected one execution on wire, got %d",
			len(
				decodedExecutionBlock.
					ReservedTransferExecutions,
			),
		)
	}

	if decodedExecutionBlock.
		ReservedTransferExecutions[0].
		ProposalID != proposal.ID {

		t.Fatal(
			"wire changed reserved transfer execution proposal ID",
		)
	}

	loadedChain, loadedPoS, _, err :=
		storage.Load(
			dataDir,
		)
	if err != nil {
		t.Fatal(err)
	}

	if !loadedChain.ValidateChain(
		loadedPoS,
	) {
		t.Fatal(
			"reloaded executed transfer chain failed validation",
		)
	}

	tip :=
		loadedChain.Blocks[len(loadedChain.Blocks)-1]

	if tip.Hash != executionBlock.Hash {
		t.Fatal(
			"storage reload changed execution block hash",
		)
	}

	if len(
		tip.ReservedTransferExecutions,
	) != 1 {
		t.Fatalf(
			"storage reload lost execution: got %d",
			len(
				tip.ReservedTransferExecutions,
			),
		)
	}

	if tip.ReservedTransferExecutions[0].
		ProposalID != proposal.ID {

		t.Fatal(
			"storage reload changed execution proposal ID",
		)
	}

	loadedGovernance, err :=
		loadedChain.GetGovernanceState()
	if err != nil {
		t.Fatal(err)
	}

	if _, exists :=
		loadedGovernance.
			GetPendingReservedTransferProposal(
				proposal.ID,
			); exists {

		t.Fatal(
			"executed transfer remained pending after reload",
		)
	}

	accounting, err :=
		loadedChain.GetReservedAccountingState()
	if err != nil {
		t.Fatal(err)
	}

	if accounting.Usage.Treasury !=
		proposal.Amount {

		t.Fatalf(
			"unexpected treasury usage: got=%d expected=%d",
			accounting.Usage.Treasury,
			proposal.Amount,
		)
	}

	afterExecution, err :=
		loadedChain.BalanceOf(
			recipient.Address,
		)
	if err != nil {
		t.Fatal(err)
	}

	expectedBalance :=
		beforeExecution +
			proposal.Amount

	if afterExecution != expectedBalance {
		t.Fatalf(
			"recipient balance mismatch: before=%d after=%d expected=%d",
			beforeExecution,
			afterExecution,
			expectedBalance,
		)
	}
}
