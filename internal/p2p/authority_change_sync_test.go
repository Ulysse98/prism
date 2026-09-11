package p2p

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/reserved"
	"prism/internal/wallet"
)

func TestAuthorityChangeStateSyncConvergesGovernance(
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

	previous := source.Blocks[len(source.Blocks)-1]
	nextHeight := previous.Height + 1

	proposer, err := pos.SelectProposer(
		previous.Hash,
		nextHeight,
	)
	if err != nil {
		t.Fatal(err)
	}

	rewardPolicy := consensus.DefaultRewardPolicy()

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

	block.Hash = blockchain.CalculateHash(
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

	sourceServer := NewServer(
		"source-governance-node",
		"127.0.0.1:7001",
		t.TempDir(),
		source,
		pos,
		map[string]*wallet.Wallet{
			"Validator":  validator,
			"AuthorityA": authorityA,
			"AuthorityB": authorityB,
		},
	)

	receiverChain := &blockchain.Blockchain{
		Blocks: []blockchain.Block{
			source.Blocks[0],
		},
		LockedStakes: map[string]uint64{
			validator.Address: stake,
		},
		Config: source.Config,
	}

	receiver := NewServer(
		"receiver-governance-node",
		"127.0.0.1:7002",
		t.TempDir(),
		receiverChain,
		pos,
		map[string]*wallet.Wallet{
			"Validator":  validator,
			"AuthorityA": authorityA,
			"AuthorityB": authorityB,
		},
	)

	if len(receiver.Chain.Blocks) != 1 {
		t.Fatalf(
			"expected receiver to start with Genesis only, got %d blocks",
			len(receiver.Chain.Blocks),
		)
	}

	beforeGovernance, err :=
		receiver.Chain.GetGovernanceState()

	if err != nil {
		t.Fatal(err)
	}

	beforeAuthorized, err :=
		beforeGovernance.CurrentPolicy.IsAuthorized(
			consensus.ReservedPoolTreasury,
			authorityB.Address,
		)

	if err != nil {
		t.Fatal(err)
	}

	if beforeAuthorized {
		t.Fatal(
			"new authority must not be active before synchronization",
		)
	}

	localConn, remoteConn := net.Pipe()

	defer localConn.Close()
	defer remoteConn.Close()

	encoder := json.NewEncoder(localConn)
	decoder := json.NewDecoder(localConn)

	remoteErr := make(chan error, 1)

	go func() {
		remoteDecoder := json.NewDecoder(
			remoteConn,
		)

		remoteEncoder := json.NewEncoder(
			remoteConn,
		)

		var request StateRequest

		if err := remoteDecoder.Decode(
			&request,
		); err != nil {
			remoteErr <- err
			return
		}

		if request.Type != MessageGetState {
			remoteErr <- &unexpectedStateRequestError{
				got: request.Type,
			}
			return
		}

		response := sourceServer.stateResponse()

		if err := remoteEncoder.Encode(
			response,
		); err != nil {
			remoteErr <- err
			return
		}

		remoteErr <- nil
	}()

	expectedRemote := sourceServer.hello()

	if err := receiver.requestAndAdoptState(
		encoder,
		decoder,
		expectedRemote,
		false,
	); err != nil {
		t.Fatal(err)
	}

	if err := <-remoteErr; err != nil {
		t.Fatal(err)
	}

	if len(receiver.Chain.Blocks) !=
		len(source.Blocks) {

		t.Fatalf(
			"expected synchronized chain length %d, got %d",
			len(source.Blocks),
			len(receiver.Chain.Blocks),
		)
	}

	sourceTip := source.Blocks[len(source.Blocks)-1]
	receiverTip :=
		receiver.Chain.Blocks[len(receiver.Chain.Blocks)-1]

	if receiverTip.Hash != sourceTip.Hash {
		t.Fatal(
			"synchronized peer tip hash does not match source",
		)
	}

	if len(receiverTip.AuthorityChanges) != 1 {
		t.Fatalf(
			"expected one synchronized authority change, got %d",
			len(receiverTip.AuthorityChanges),
		)
	}

	receivedChange :=
		receiverTip.AuthorityChanges[0]

	if receivedChange.ID != change.ID {
		t.Fatal(
			"synchronized authority change ID differs from source",
		)
	}

	if len(receivedChange.Approvals) !=
		len(change.Approvals) {

		t.Fatalf(
			"expected %d synchronized approvals, got %d",
			len(change.Approvals),
			len(receivedChange.Approvals),
		)
	}

	if receivedChange.Approvals[0].Signature !=
		change.Approvals[0].Signature {

		t.Fatal(
			"synchronized authority-change signature differs from source",
		)
	}

	governance, err :=
		receiver.Chain.GetGovernanceState()

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
			"synchronized peer did not reconstruct new authority",
		)
	}

	if !receiver.Chain.ValidateChain(pos) {
		t.Fatal(
			"synchronized authority-change chain failed validation",
		)
	}
}

type unexpectedStateRequestError struct {
	got string
}

func (err *unexpectedStateRequestError) Error() string {
	return "unexpected state request type: " + err.got
}

func TestTimelockedAuthorityChangeStateSyncRejectedBeforeActivation(
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

	chainID, err := source.ChainID()
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
		blockchain.CalculateHash(block)

	source.Blocks = append(
		source.Blocks,
		block,
	)

	// The source deliberately contains an invalid governance transition:
	// height 1 attempts to activate a change scheduled for height 2.
	if source.ValidateChain(pos) {
		t.Fatal(
			"expected premature timelocked source chain to be invalid",
		)
	}

	sourceServer := NewServer(
		"timelock-source-node",
		"127.0.0.1:7001",
		t.TempDir(),
		source,
		pos,
		map[string]*wallet.Wallet{
			"Validator":  validator,
			"AuthorityA": authorityA,
			"AuthorityB": authorityB,
		},
	)

	receiverChain := &blockchain.Blockchain{
		Blocks: []blockchain.Block{
			source.Blocks[0],
		},
		LockedStakes: map[string]uint64{
			validator.Address: stake,
		},
		Config: source.Config,
	}

	receiver := NewServer(
		"timelock-receiver-node",
		"127.0.0.1:7002",
		t.TempDir(),
		receiverChain,
		pos,
		map[string]*wallet.Wallet{
			"Validator":  validator,
			"AuthorityA": authorityA,
			"AuthorityB": authorityB,
		},
	)

	beforeLength :=
		len(receiver.Chain.Blocks)

	localConn, remoteConn :=
		net.Pipe()

	defer localConn.Close()
	defer remoteConn.Close()

	encoder :=
		json.NewEncoder(localConn)

	decoder :=
		json.NewDecoder(localConn)

	remoteErr :=
		make(chan error, 1)

	go func() {
		remoteDecoder :=
			json.NewDecoder(remoteConn)

		remoteEncoder :=
			json.NewEncoder(remoteConn)

		var request StateRequest

		if err := remoteDecoder.Decode(
			&request,
		); err != nil {
			remoteErr <- err
			return
		}

		if request.Type != MessageGetState {
			remoteErr <- &unexpectedStateRequestError{
				got: request.Type,
			}
			return
		}

		response :=
			sourceServer.stateResponse()

		if err := remoteEncoder.Encode(
			response,
		); err != nil {
			remoteErr <- err
			return
		}

		remoteErr <- nil
	}()

	expectedRemote :=
		sourceServer.hello()

	err =
		receiver.requestAndAdoptState(
			encoder,
			decoder,
			expectedRemote,
			false,
		)

	if err == nil {
		t.Fatal(
			"expected state sync with premature timelocked authority change to fail",
		)
	}

	if err := <-remoteErr; err != nil {
		t.Fatal(err)
	}

	if len(receiver.Chain.Blocks) != beforeLength {
		t.Fatalf(
			"rejected state sync changed receiver chain length: before=%d after=%d",
			beforeLength,
			len(receiver.Chain.Blocks),
		)
	}

	if !receiver.Chain.ValidateChain(pos) {
		t.Fatal(
			"rejected state sync invalidated receiver chain",
		)
	}

	governance, err :=
		receiver.Chain.GetGovernanceState()

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
			"rejected state sync activated premature authority change",
		)
	}
}
