package p2p

import (
	"strings"
	"testing"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/reserved"
)

func p2pConfigCommitmentFixture(
	t *testing.T,
	config blockchain.ChainConfig,
) (
	*Server,
	*blockchain.Blockchain,
) {
	t.Helper()

	chain, err :=
		blockchain.NewBlockchain(
			map[string]uint64{
				"alice": 1000,
			},
		)

	if err != nil {
		t.Fatal(err)
	}

	chain.Config =
		config

	server :=
		NewServer(
			"config-test-node",
			"127.0.0.1:7001",
			t.TempDir(),
			chain,
			consensus.NewProofOfStake(),
			nil,
		)

	return server, chain
}

func TestLegacyHelloPreservesLegacyNetworkIdentity(
	t *testing.T,
) {
	server, chain :=
		p2pConfigCommitmentFixture(
			t,
			blockchain.DefaultChainConfig(),
		)

	hello :=
		server.hello()

	if hello.ConfigCommitment != "" {
		t.Fatal(
			"legacy hello must not advertise a config commitment",
		)
	}

	expected :=
		blockchain.MakeChainID(
			chain.Blocks[0].Hash,
		)

	if hello.ChainID != expected {
		t.Fatalf(
			"expected legacy Chain ID %s, got %s",
			expected,
			hello.ChainID,
		)
	}

	if err :=
		validateHello(
			hello,
		); err != nil {

		t.Fatal(err)
	}
}

func TestConfiguredHelloBindsConfigCommitment(
	t *testing.T,
) {
	config :=
		blockchain.ChainConfig{
			ReservedAuthorities: reserved.AuthorityPolicy{
				Treasury: []string{
					"prism_authority_a",
				},
			},
		}

	server, chain :=
		p2pConfigCommitmentFixture(
			t,
			config,
		)

	hello :=
		server.hello()

	commitment, err :=
		config.Commitment()

	if err != nil {
		t.Fatal(err)
	}

	if hello.ConfigCommitment !=
		commitment {

		t.Fatal(
			"hello did not advertise config commitment",
		)
	}

	expected, err :=
		blockchain.MakeChainIDFromConfigCommitment(
			chain.Blocks[0].Hash,
			commitment,
		)

	if err != nil {
		t.Fatal(err)
	}

	if hello.ChainID != expected {
		t.Fatal(
			"hello Chain ID is not bound to config commitment",
		)
	}

	if err :=
		validateHello(
			hello,
		); err != nil {

		t.Fatal(err)
	}
}

func TestValidateHelloRejectsTamperedConfigCommitment(
	t *testing.T,
) {
	config :=
		blockchain.ChainConfig{
			ReservedAuthorities: reserved.AuthorityPolicy{
				Treasury: []string{
					"prism_authority_a",
				},
			},
		}

	server, _ :=
		p2pConfigCommitmentFixture(
			t,
			config,
		)

	hello :=
		server.hello()

	hello.ConfigCommitment =
		strings.Repeat(
			"a",
			64,
		)

	if err :=
		validateHello(
			hello,
		); err == nil {

		t.Fatal(
			"expected tampered config commitment to fail",
		)
	}
}

func TestValidateHelloRejectsMalformedConfigCommitment(
	t *testing.T,
) {
	server, _ :=
		p2pConfigCommitmentFixture(
			t,
			blockchain.DefaultChainConfig(),
		)

	hello :=
		server.hello()

	hello.ConfigCommitment =
		"not-a-sha256"

	if err :=
		validateHello(
			hello,
		); err == nil {

		t.Fatal(
			"expected malformed config commitment to fail",
		)
	}
}

func TestStateResponseBindsBlockchainConfig(
	t *testing.T,
) {
	config :=
		blockchain.ChainConfig{
			ReservedAuthorities: reserved.AuthorityPolicy{
				Treasury: []string{
					"prism_authority_a",
				},
			},
		}

	server, chain :=
		p2pConfigCommitmentFixture(
			t,
			config,
		)

	response :=
		server.stateResponse()

	commitment, err :=
		config.Commitment()

	if err != nil {
		t.Fatal(err)
	}

	if response.ConfigCommitment !=
		commitment {

		t.Fatal(
			"state response config commitment mismatch",
		)
	}

	expected, err :=
		blockchain.MakeChainIDFromConfigCommitment(
			chain.Blocks[0].Hash,
			commitment,
		)

	if err != nil {
		t.Fatal(err)
	}

	if response.ChainID != expected {
		t.Fatal(
			"state response Chain ID is not bound to config",
		)
	}
}

func TestNetworkIdentityRejectsInvalidConfig(
	t *testing.T,
) {
	chain, err :=
		blockchain.NewBlockchain(
			map[string]uint64{
				"alice": 1000,
			},
		)

	if err != nil {
		t.Fatal(err)
	}

	chain.Config =
		blockchain.ChainConfig{
			ReservedAuthorities: reserved.AuthorityPolicy{
				Treasury: []string{
					"",
				},
			},
		}

	if _, _, err :=
		networkIdentity(
			chain,
		); err == nil {

		t.Fatal(
			"expected invalid config network identity to fail",
		)
	}
}
