package p2p

import (
	"encoding/json"
	"testing"
	"time"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/reserved"
	"prism/internal/wallet"
)

func TestAuthorityChangeBlockConvergesAcrossPeers(
	t *testing.T,
) {
	validator, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	authorityA, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	authorityB, err := wallet.New()
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
				authorityA.Address,
			},
		},
	}

	makeReceiver := func(
		nodeID string,
	) *Server {
		chain := &blockchain.Blockchain{
			Blocks: append(
				[]blockchain.Block(nil),
				source.Blocks...,
			),
			LockedStakes: map[string]uint64{
				validator.Address: stake,
			},
			Config: source.Config,
		}

		return NewServer(
			nodeID,
			"127.0.0.1:0",
			t.TempDir(),
			chain,
			pos,
			map[string]*wallet.Wallet{
				"Validator":  validator,
				"AuthorityA": authorityA,
				"AuthorityB": authorityB,
			},
		)
	}

	nodeB := makeReceiver(
		"governance-node-b",
	)

	nodeC := makeReceiver(
		"governance-node-c",
	)

	chainID, err := source.ChainID()
	if err != nil {
		t.Fatal(err)
	}

	change := reserved.NewAuthorityChange(
		chainID,
		1,
		consensus.ReservedPoolTreasury,
		reserved.AuthorityChangeAdd,
		authorityB.Address,
	)

	if err := change.AddApproval(
		authorityA.Address,
		authorityA.PublicKeyHex(),
		authorityA.PrivateKey,
	); err != nil {
		t.Fatal(err)
	}

	previous :=
		source.Blocks[len(source.Blocks)-1]

	nextHeight :=
		previous.Height + 1

	proposer, err := pos.SelectProposer(
		previous.Hash,
		nextHeight,
	)
	if err != nil {
		t.Fatal(err)
	}

	rewardPolicy :=
		consensus.DefaultRewardPolicy()

	block := blockchain.Block{
		Height:       nextHeight,
		Timestamp:    time.Unix(2, 0).UTC(),
		PreviousHash: previous.Hash,
		Proposer:     proposer.Address,
		Reward:       rewardPolicy.ProposerReward,
		AuthorityChanges: []reserved.AuthorityChange{
			change,
		},
	}

	block.Hash =
		blockchain.CalculateHash(
			block,
		)

	source.Blocks = append(
		source.Blocks,
		block,
	)

	if !source.ValidateChain(pos) {
		t.Fatal(
			"source authority-change chain failed validation",
		)
	}

	message := BlockMessage{
		Type:    MessageBlock,
		ChainID: chainID,
		Block:   block,
	}

	wire, err :=
		json.Marshal(
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

	if len(decoded.Block.AuthorityChanges) != 1 {
		t.Fatalf(
			"expected one authority change on wire, got %d",
			len(decoded.Block.AuthorityChanges),
		)
	}

	wireChange :=
		decoded.Block.AuthorityChanges[0]

	if wireChange.ID != change.ID {
		t.Fatal(
			"wire encoding changed authority change ID",
		)
	}

	if wireChange.Authority != authorityB.Address {
		t.Fatal(
			"wire encoding changed target authority",
		)
	}

	if len(wireChange.Approvals) != 1 {
		t.Fatalf(
			"expected one authority-change approval, got %d",
			len(wireChange.Approvals),
		)
	}

	if wireChange.Approvals[0].Authorizer !=
		change.Approvals[0].Authorizer {

		t.Fatal(
			"wire encoding changed authority-change approver",
		)
	}

	if wireChange.Approvals[0].Signature !=
		change.Approvals[0].Signature {

		t.Fatal(
			"wire encoding changed authority-change signature",
		)
	}

	for _, server := range []*Server{
		nodeB,
		nodeC,
	} {
		appended, err :=
			server.acceptBlock(
				decoded.Block,
			)

		if err != nil {
			t.Fatal(err)
		}

		if !appended {
			t.Fatal(
				"expected authority-change block to append",
			)
		}

		tip := server.Chain.Blocks[len(server.Chain.Blocks)-1]

		if tip.Hash != block.Hash {
			t.Fatal(
				"peer changed authority-change block hash",
			)
		}

		if len(tip.AuthorityChanges) != 1 {
			t.Fatalf(
				"expected one authority change on peer, got %d",
				len(tip.AuthorityChanges),
			)
		}

		received :=
			tip.AuthorityChanges[0]

		if received.ID != change.ID {
			t.Fatal(
				"peer changed authority-change ID",
			)
		}

		if received.ChainID != change.ChainID {
			t.Fatal(
				"peer changed authority-change Chain ID",
			)
		}

		if received.Nonce != change.Nonce {
			t.Fatal(
				"peer changed authority-change nonce",
			)
		}

		if received.Pool != change.Pool {
			t.Fatal(
				"peer changed authority-change pool",
			)
		}

		if received.Action != change.Action {
			t.Fatal(
				"peer changed authority-change action",
			)
		}

		if received.Authority != change.Authority {
			t.Fatal(
				"peer changed target authority",
			)
		}

		if len(received.Approvals) != 1 {
			t.Fatalf(
				"expected one received authority-change approval, got %d",
				len(received.Approvals),
			)
		}

		if received.Approvals[0].Authorizer !=
			change.Approvals[0].Authorizer {

			t.Fatal(
				"peer changed authority-change approver",
			)
		}

		if received.Approvals[0].Signature !=
			change.Approvals[0].Signature {

			t.Fatal(
				"peer changed authority-change signature",
			)
		}

		governance, err :=
			server.Chain.GetGovernanceState()

		if err != nil {
			t.Fatal(err)
		}

		authorized, err :=
			governance.CurrentPolicy.IsAuthorized(
				consensus.ReservedPoolTreasury,
				authorityB.Address,
			)

		if err != nil {
			t.Fatal(err)
		}

		if !authorized {
			t.Fatal(
				"peer did not activate received authority change",
			)
		}

		if !server.Chain.ValidateChain(pos) {
			t.Fatal(
				"peer authority-change chain failed validation",
			)
		}
	}
}

func TestTimelockedAuthorityChangeBlockRejectedBeforeActivation(
	t *testing.T,
) {
	validator, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	authorityA, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	authorityB, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	chain, err := blockchain.NewBlockchain(
		map[string]uint64{
			validator.Address: 1000,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	const stake uint64 = 10

	if err := chain.LockStake(
		validator.Address,
		stake,
	); err != nil {
		t.Fatal(err)
	}

	pos :=
		consensus.NewProofOfStake()

	if err := pos.Register(
		validator.Address,
		stake,
	); err != nil {
		t.Fatal(err)
	}

	chain.Config = blockchain.ChainConfig{
		ReservedAuthorities: reserved.AuthorityPolicy{
			Treasury: []string{
				authorityA.Address,
			},
		},
	}

	server := NewServer(
		"timelock-receiver",
		"127.0.0.1:0",
		t.TempDir(),
		chain,
		pos,
		map[string]*wallet.Wallet{
			"Validator":  validator,
			"AuthorityA": authorityA,
			"AuthorityB": authorityB,
		},
	)

	chainID, err :=
		chain.ChainID()

	if err != nil {
		t.Fatal(err)
	}

	change :=
		reserved.NewTimelockedAuthorityChange(
			chainID,
			1,
			consensus.ReservedPoolTreasury,
			reserved.AuthorityChangeAdd,
			authorityB.Address,
			2,
		)

	if err := change.AddApproval(
		authorityA.Address,
		authorityA.PublicKeyHex(),
		authorityA.PrivateKey,
	); err != nil {
		t.Fatal(err)
	}

	previous :=
		chain.Blocks[len(chain.Blocks)-1]

	nextHeight :=
		previous.Height + 1

	proposer, err :=
		pos.SelectProposer(
			previous.Hash,
			nextHeight,
		)

	if err != nil {
		t.Fatal(err)
	}

	rewardPolicy :=
		consensus.DefaultRewardPolicy()

	block := blockchain.Block{
		Height:       nextHeight,
		Timestamp:    time.Unix(2, 0).UTC(),
		PreviousHash: previous.Hash,
		Proposer:     proposer.Address,
		Reward:       rewardPolicy.ProposerReward,
		AuthorityChanges: []reserved.AuthorityChange{
			change,
		},
	}

	block.Hash =
		blockchain.CalculateHash(
			block,
		)

	beforeLength :=
		len(server.Chain.Blocks)

	appended, err :=
		server.acceptBlock(
			block,
		)

	if err == nil {
		t.Fatal(
			"expected premature timelocked authority change block to be rejected",
		)
	}

	if appended {
		t.Fatal(
			"premature timelocked block must not be appended",
		)
	}

	if len(server.Chain.Blocks) != beforeLength {
		t.Fatalf(
			"rejected block changed chain length: before=%d after=%d",
			beforeLength,
			len(server.Chain.Blocks),
		)
	}

	if !server.Chain.ValidateChain(pos) {
		t.Fatal(
			"rejecting premature timelocked block invalidated local chain",
		)
	}

	governance, err :=
		server.Chain.GetGovernanceState()

	if err != nil {
		t.Fatal(err)
	}

	authorized, err :=
		governance.CurrentPolicy.IsAuthorized(
			consensus.ReservedPoolTreasury,
			authorityB.Address,
		)

	if err != nil {
		t.Fatal(err)
	}

	if authorized {
		t.Fatal(
			"premature timelocked authority change mutated governance state",
		)
	}
}
