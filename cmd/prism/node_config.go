package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"prism/internal/blockchain"
)

func loadNodeChainConfig(
	path string,
) (
	*blockchain.ChainConfig,
	error,
) {
	if path == "" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot read chain config: %w",
			err,
		)
	}

	if bytes.Equal(
		bytes.TrimSpace(data),
		[]byte("null"),
	) {
		return nil, fmt.Errorf(
			"chain config must be a JSON object",
		)
	}

	decoder :=
		json.NewDecoder(
			bytes.NewReader(data),
		)

	decoder.DisallowUnknownFields()

	var config blockchain.ChainConfig

	if err := decoder.Decode(
		&config,
	); err != nil {
		return nil, fmt.Errorf(
			"cannot decode chain config: %w",
			err,
		)
	}

	var extra any

	if err := decoder.Decode(
		&extra,
	); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf(
				"chain config contains trailing JSON",
			)
		}

		return nil, fmt.Errorf(
			"invalid trailing chain config data: %w",
			err,
		)
	}

	canonical, err :=
		config.Canonical()

	if err != nil {
		return nil, fmt.Errorf(
			"invalid chain config: %w",
			err,
		)
	}

	return &canonical, nil
}

func ensureNodeChainConfig(
	chain *blockchain.Blockchain,
	requested *blockchain.ChainConfig,
) error {
	if requested == nil {
		return nil
	}

	if chain == nil {
		return fmt.Errorf(
			"blockchain cannot be nil",
		)
	}

	currentCanonical, err :=
		chain.Config.Canonical()

	if err != nil {
		return fmt.Errorf(
			"stored chain config is invalid: %w",
			err,
		)
	}

	requestedCanonical, err :=
		requested.Canonical()

	if err != nil {
		return fmt.Errorf(
			"requested chain config is invalid: %w",
			err,
		)
	}

	currentCommitment, err :=
		currentCanonical.Commitment()

	if err != nil {
		return err
	}

	requestedCommitment, err :=
		requestedCanonical.Commitment()

	if err != nil {
		return err
	}

	if currentCommitment !=
		requestedCommitment {

		return fmt.Errorf(
			"stored chain config does not match requested chain config",
		)
	}

	return nil
}
