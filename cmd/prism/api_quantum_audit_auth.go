package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// loadQuantumAuditToken accepts exactly 32 bytes encoded as
// 64 hex characters, optionally followed by a newline.
func loadQuantumAuditToken(path string) ([32]byte, error) {
	var empty [32]byte

	info, err := os.Lstat(path)
	if err != nil {
		return empty, err
	}

	if !info.Mode().IsRegular() ||
		info.Size() < 64 ||
		info.Size() > 128 {
		return empty, fmt.Errorf("invalid quantum audit token file")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return empty, err
	}

	value := strings.TrimSpace(string(data))

	if len(value) != 64 {
		return empty, fmt.Errorf("invalid quantum audit token length")
	}

	token, err := hex.DecodeString(value)
	if err != nil || len(token) != 32 {
		return empty, fmt.Errorf("invalid quantum audit token encoding")
	}

	return sha256.Sum256(token), nil
}

// Compare token hashes in constant time without logging secrets.
func verifyQuantumAuditBearer(
	request *http.Request,
	expected *[32]byte,
) bool {
	if request == nil || expected == nil {
		return false
	}

	values := request.Header.Values("Authorization")
	if len(values) != 1 {
		return false
	}

	const prefix = "Bearer "
	authorization := values[0]

	if !strings.HasPrefix(authorization, prefix) {
		return false
	}

	encoded := strings.TrimPrefix(authorization, prefix)
	if len(encoded) != 64 {
		return false
	}

	token, err := hex.DecodeString(encoded)
	if err != nil || len(token) != 32 {
		return false
	}

	actual := sha256.Sum256(token)

	return subtle.ConstantTimeCompare(
		actual[:],
		expected[:],
	) == 1
}
