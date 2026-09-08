package p2p

import (
	"net"
	"testing"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/identity"
	"prism/internal/participation"
	"prism/internal/poup"
	"prism/internal/reserved"
	"prism/internal/transaction"
	"prism/internal/usefulwork"
	"prism/internal/wallet"
)

func TestReachablePeerAddressReplacesWildcardHost(t *testing.T) {
	remote := &net.TCPAddr{
		IP:   net.ParseIP("172.20.0.3"),
		Port: 54321,
	}

	got := reachablePeerAddress(
		"0.0.0.0:7003",
		remote,
	)

	if got != "172.20.0.3:7003" {
		t.Fatalf(
			"expected 172.20.0.3:7003, got %s",
			got,
		)
	}
}

func TestReachablePeerAddressPreservesAdvertisedHost(t *testing.T) {
	remote := &net.TCPAddr{
		IP:   net.ParseIP("172.20.0.3"),
		Port: 54321,
	}

	got := reachablePeerAddress(
		"prism-node-3:7003",
		remote,
	)

	if got != "prism-node-3:7003" {
		t.Fatalf(
			"expected advertised address to survive, got %s",
			got,
		)
	}
}

func TestNewServerInitializesMempool(t *testing.T) {
	server := NewServer(
		"node-test",
		"127.0.0.1:7001",
		"data/test",
		nil,
		nil,
		nil,
	)

	if server.Pool == nil {
		t.Fatal("expected server mempool to be initialized")
	}

	if server.MempoolCount() != 0 {
		t.Fatal("expected empty server mempool")
	}
}

func makeBlockAcceptanceFixture(
	t *testing.T,
) (
	*Server,
	blockchain.Block,
) {
	t.Helper()

	alice, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	bob, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	source, err := blockchain.NewBlockchain(
		map[string]uint64{
			alice.Address: 100,
			bob.Address:   10,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := source.LockStake(
		alice.Address,
		10,
	); err != nil {
		t.Fatal(err)
	}

	pos := consensus.NewProofOfStake()

	if err := pos.Register(
		alice.Address,
		10,
	); err != nil {
		t.Fatal(err)
	}

	receiver := &blockchain.Blockchain{
		Blocks: append(
			[]blockchain.Block(nil),
			source.Blocks...,
		),
		LockedStakes: map[string]uint64{
			alice.Address: 10,
		},
	}

	tx := transaction.New(
		alice.Address,
		bob.Address,
		1,
		0,
		alice.PublicKeyHex(),
	)

	if err := tx.Sign(
		alice.PrivateKey,
	); err != nil {
		t.Fatal(err)
	}

	block, err := source.AddBlock(
		[]transaction.Transaction{
			tx,
		},
		nil,
		alice.Address,
		pos,
	)
	if err != nil {
		t.Fatal(err)
	}

	server := NewServer(
		"receiver-node",
		"127.0.0.1:7002",
		t.TempDir(),
		receiver,
		pos,
		map[string]*wallet.Wallet{
			"Alice": alice,
			"Bob":   bob,
		},
	)

	return server, block
}

func TestAcceptBlockTreatsExactDuplicateAsKnown(
	t *testing.T,
) {
	server, block :=
		makeBlockAcceptanceFixture(t)

	appended, err := server.acceptBlock(
		block,
	)
	if err != nil {
		t.Fatal(err)
	}

	if !appended {
		t.Fatal(
			"expected first block submission to append",
		)
	}

	appended, err = server.acceptBlock(
		block,
	)
	if err != nil {
		t.Fatal(err)
	}

	if appended {
		t.Fatal(
			"expected duplicate block to be treated as already known",
		)
	}

	if len(server.Chain.Blocks) != 2 {
		t.Fatalf(
			"expected exactly 2 blocks, got %d",
			len(server.Chain.Blocks),
		)
	}
}

func TestAcceptBlockRejectsConflictingKnownHeight(
	t *testing.T,
) {
	server, block :=
		makeBlockAcceptanceFixture(t)

	appended, err := server.acceptBlock(
		block,
	)
	if err != nil {
		t.Fatal(err)
	}

	if !appended {
		t.Fatal(
			"expected first block submission to append",
		)
	}

	conflict := block
	conflict.Hash =
		"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"

	appended, err = server.acceptBlock(
		conflict,
	)

	if err == nil {
		t.Fatal(
			"expected conflicting block to be rejected",
		)
	}

	if appended {
		t.Fatal(
			"conflicting block must not append",
		)
	}

	if len(server.Chain.Blocks) != 2 {
		t.Fatalf(
			"conflict mutated chain: got %d blocks",
			len(server.Chain.Blocks),
		)
	}
}

func TestPoUPClaimBlockConvergesAcrossPeers(
	t *testing.T,
) {
	actor, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	source, err := blockchain.NewBlockchain(
		map[string]uint64{
			actor.Address: 1000,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	const stake uint64 = 100

	if err := source.LockStake(
		actor.Address,
		stake,
	); err != nil {
		t.Fatal(err)
	}

	pos := consensus.NewProofOfStake()

	if err := pos.Register(
		actor.Address,
		stake,
	); err != nil {
		t.Fatal(err)
	}

	attestation, err :=
		identity.NewWorldIDAttestation(
			actor.Address,
			"p2p-poup-nullifier",
			"prism-p2p-poup",
		)

	if err != nil {
		t.Fatal(err)
	}

	// Build one complete PoUP reward period.
	for index := 1; index <= 100; index++ {
		task, err :=
			usefulwork.NewSumSquaresTask(
				[]uint64{
					uint64(index),
				},
			)

		if err != nil {
			t.Fatal(err)
		}

		proof, err :=
			usefulwork.Execute(
				task,
				actor,
			)

		if err != nil {
			t.Fatal(err)
		}

		if index == 1 {
			if _, err := source.AddHumanityBlock(
				[]identity.Attestation{
					attestation,
				},
				actor.Address,
				pos,
			); err != nil {
				t.Fatal(err)
			}

			// Humanity block consumed height 1, so useful work
			// starts at height 2 in this fixture.
			continue
		}

		if _, err := source.AddBlock(
			nil,
			[]usefulwork.Proof{
				proof,
			},
			actor.Address,
			pos,
		); err != nil {
			t.Fatal(err)
		}
	}

	// We need period 0 to contain 100 activity blocks.
	task, err :=
		usefulwork.NewSumSquaresTask(
			[]uint64{101},
		)
	if err != nil {
		t.Fatal(err)
	}

	proof, err :=
		usefulwork.Execute(
			task,
			actor,
		)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := source.AddBlock(
		nil,
		[]usefulwork.Proof{
			proof,
		},
		actor.Address,
		pos,
	); err != nil {
		t.Fatal(err)
	}

	// Clone A's pre-claim chain into B and C.
	makeReceiver := func(
		nodeID string,
	) *Server {
		chain := &blockchain.Blockchain{
			Blocks: append(
				[]blockchain.Block(nil),
				source.Blocks...,
			),
			LockedStakes: map[string]uint64{
				actor.Address: stake,
			},
		}

		return NewServer(
			nodeID,
			"127.0.0.1:0",
			t.TempDir(),
			chain,
			pos,
			map[string]*wallet.Wallet{
				"Actor": actor,
			},
		)
	}

	nodeB := makeReceiver("node-b")
	nodeC := makeReceiver("node-c")

	reward, err :=
		participation.EvaluatePeriodReward(
			source,
			pos,
			source,
			actor.Address,
			0,
		)

	if err != nil {
		t.Fatal(err)
	}

	if reward.Amount == 0 {
		t.Fatal(
			"expected rewardable PoUP period",
		)
	}

	claim := poup.NewClaim(
		actor.Address,
		0,
		reward.Points,
		reward.Units,
		reward.Amount,
		actor.PublicKeyHex(),
	)

	if err := claim.Sign(
		actor.PrivateKey,
	); err != nil {
		t.Fatal(err)
	}

	block, err :=
		source.AddParticipationClaimBlock(
			[]poup.Claim{
				claim,
			},
			actor.Address,
			pos,
		)

	if err != nil {
		t.Fatal(err)
	}

	for _, server := range []*Server{
		nodeB,
		nodeC,
	} {
		appended, err :=
			server.acceptBlock(block)

		if err != nil {
			t.Fatal(err)
		}

		if !appended {
			t.Fatal(
				"expected PoUP block to append",
			)
		}

		tip := server.Chain.Blocks[len(server.Chain.Blocks)-1]

		if tip.Hash != block.Hash {
			t.Fatal(
				"peer changed PoUP block hash",
			)
		}

		if len(tip.ParticipationClaims) != 1 {
			t.Fatal(
				"peer lost PoUP claim",
			)
		}

		if tip.ParticipationClaims[0].ID !=
			claim.ID {

			t.Fatal(
				"peer changed PoUP claim ID",
			)
		}

		if tip.ParticipationClaims[0].Signature !=
			claim.Signature {

			t.Fatal(
				"peer changed PoUP claim signature",
			)
		}

		if !server.Chain.ValidateChain(pos) {
			t.Fatal(
				"peer PoUP chain failed validation",
			)
		}
	}
}

func TestReservedAuthorizationBlockConvergesAcrossPeers(
	t *testing.T,
) {
	validator, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	authorizer, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	recipient, err := wallet.New()
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
				authorizer.Address,
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
				"Authorizer": authorizer,
				"Recipient":  recipient,
			},
		)
	}

	nodeB := makeReceiver("reserved-node-b")
	nodeC := makeReceiver("reserved-node-c")

	chainID, err := source.ChainID()
	if err != nil {
		t.Fatal(err)
	}

	authorization :=
		reserved.NewAuthorization(
			chainID,
			1,
			consensus.ReservedPoolTreasury,
			recipient.Address,
			100,
			authorizer.Address,
			authorizer.PublicKeyHex(),
		)

	if err := authorization.Sign(
		authorizer.PrivateKey,
	); err != nil {
		t.Fatal(err)
	}

	block, err :=
		source.AddReservedAuthorizationBlock(
			[]reserved.Authorization{
				authorization,
			},
			validator.Address,
			pos,
		)

	if err != nil {
		t.Fatal(err)
	}

	for _, server := range []*Server{
		nodeB,
		nodeC,
	} {
		appended, err :=
			server.acceptBlock(block)

		if err != nil {
			t.Fatal(err)
		}

		if !appended {
			t.Fatal(
				"expected reserved block to append",
			)
		}

		tip :=
			server.Chain.Blocks[len(server.Chain.Blocks)-1]

		if tip.Hash != block.Hash {
			t.Fatal(
				"peer changed reserved block hash",
			)
		}

		if len(tip.ReservedAuthorizations) != 1 {
			t.Fatal(
				"peer lost reserved authorization",
			)
		}

		received :=
			tip.ReservedAuthorizations[0]

		if received.ID != authorization.ID {
			t.Fatal(
				"peer changed reserved authorization ID",
			)
		}

		if received.Signature != authorization.Signature {
			t.Fatal(
				"peer changed reserved authorization signature",
			)
		}

		balance, err :=
			server.Chain.BalanceOf(
				recipient.Address,
			)

		if err != nil {
			t.Fatal(err)
		}

		if balance != authorization.Amount {
			t.Fatalf(
				"expected reserved balance %d, got %d",
				authorization.Amount,
				balance,
			)
		}

		emission, err :=
			server.Chain.ReservedEmission()

		if err != nil {
			t.Fatal(err)
		}

		if emission != authorization.Amount {
			t.Fatalf(
				"expected reserved emission %d, got %d",
				authorization.Amount,
				emission,
			)
		}

		if !server.Chain.ValidateChain(pos) {
			t.Fatal(
				"peer reserved chain failed validation",
			)
		}
	}
}

func TestThresholdReservedGrantBlockConvergesAcrossPeers(
	t *testing.T,
) {
	validator, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	recipient, err := wallet.New()
	if err != nil {
		t.Fatal(err)
	}

	authorities := make(
		[]*wallet.Wallet,
		3,
	)

	addresses := make(
		[]string,
		3,
	)

	for i := range authorities {
		authorities[i], err =
			wallet.New()

		if err != nil {
			t.Fatal(err)
		}

		addresses[i] =
			authorities[i].Address
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
			Treasury:          addresses,
			TreasuryThreshold: 2,
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
				"Recipient":  recipient,
				"AuthorityA": authorities[0],
				"AuthorityB": authorities[1],
				"AuthorityC": authorities[2],
			},
		)
	}

	nodeB :=
		makeReceiver(
			"threshold-reserved-node-b",
		)

	nodeC :=
		makeReceiver(
			"threshold-reserved-node-c",
		)

	chainID, err :=
		source.ChainID()

	if err != nil {
		t.Fatal(err)
	}

	grant :=
		reserved.NewGrant(
			chainID,
			1,
			consensus.ReservedPoolTreasury,
			recipient.Address,
			100,
		)

	for _, authority := range []*wallet.Wallet{
		authorities[0],
		authorities[1],
	} {
		if err :=
			grant.AddApproval(
				authority.Address,
				authority.PublicKeyHex(),
				authority.PrivateKey,
			); err != nil {

			t.Fatal(err)
		}
	}

	block, err :=
		source.AddReservedGrantBlock(
			[]reserved.Grant{
				grant,
			},
			validator.Address,
			pos,
		)

	if err != nil {
		t.Fatal(err)
	}

	if len(block.ReservedGrants) != 1 {
		t.Fatalf(
			"expected source block to contain one reserved grant, got %d",
			len(block.ReservedGrants),
		)
	}

	if len(block.ReservedGrants[0].Approvals) != 2 {
		t.Fatalf(
			"expected source grant to contain two approvals, got %d",
			len(block.ReservedGrants[0].Approvals),
		)
	}

	for _, server := range []*Server{
		nodeB,
		nodeC,
	} {
		appended, err :=
			server.acceptBlock(
				block,
			)

		if err != nil {
			t.Fatal(err)
		}

		if !appended {
			t.Fatal(
				"expected threshold reserved grant block to append",
			)
		}

		tip := server.Chain.Blocks[len(server.Chain.Blocks)-1]

		if tip.Hash != block.Hash {
			t.Fatal(
				"peer changed threshold reserved grant block hash",
			)
		}

		if len(tip.ReservedGrants) != 1 {
			t.Fatalf(
				"expected peer to preserve one reserved grant, got %d",
				len(tip.ReservedGrants),
			)
		}

		received :=
			tip.ReservedGrants[0]

		if received.ID != grant.ID {
			t.Fatal(
				"peer changed reserved grant ID",
			)
		}

		if len(received.Approvals) !=
			len(grant.Approvals) {

			t.Fatalf(
				"expected %d grant approvals, got %d",
				len(grant.Approvals),
				len(received.Approvals),
			)
		}

		for i := range grant.Approvals {

			if received.Approvals[i].Authorizer !=
				grant.Approvals[i].Authorizer {

				t.Fatalf(
					"peer changed grant approver at index %d",
					i,
				)
			}

			if received.Approvals[i].Signature !=
				grant.Approvals[i].Signature {

				t.Fatalf(
					"peer changed grant signature at index %d",
					i,
				)
			}
		}

		balance, err :=
			server.Chain.BalanceOf(
				recipient.Address,
			)

		if err != nil {
			t.Fatal(err)
		}

		if balance != grant.Amount {
			t.Fatalf(
				"expected threshold reserved balance %d, got %d",
				grant.Amount,
				balance,
			)
		}

		emission, err :=
			server.Chain.ReservedEmission()

		if err != nil {
			t.Fatal(err)
		}

		if emission != grant.Amount {
			t.Fatalf(
				"expected threshold reserved emission %d, got %d",
				grant.Amount,
				emission,
			)
		}

		if !server.Chain.ValidateChain(pos) {
			t.Fatal(
				"peer threshold reserved grant chain failed validation",
			)
		}
	}
}
