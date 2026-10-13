package p2p

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/storage"
	"prism/internal/wallet"
)

// persistPeerStateV061 chooses the existing state format without ever
// writing wallets.json in a public-only observer directory.
// The ordinary private node storage path retains its existing behavior.
func persistPeerStateV061(dataDir string, chain *blockchain.Blockchain, pos *consensus.ProofOfStake, wallets map[string]*wallet.Wallet) error {
	privatePath := filepath.Join(dataDir, "wallets.json")
	publicPath := filepath.Join(dataDir, "api-wallets.json")

	hasPrivate, err := regularFileExistsV061(privatePath)
	if err != nil {
		return err
	}
	hasPublic, err := regularFileExistsV061(publicPath)
	if err != nil {
		return err
	}

	if hasPrivate && hasPublic {
		return fmt.Errorf("ambiguous peer data: both public and private wallet files present")
	}
	if hasPublic {
		if !storage.ExistsPublic(dataDir) {
			return fmt.Errorf("incomplete public peer state")
		}
		for name, w := range wallets {
			if w == nil || len(w.PrivateKey) != 0 {
				return fmt.Errorf("public peer has private key material or invalid wallet: %s", name)
			}
		}
		return storage.SavePublicChain(dataDir, chain, pos)
	}
	if hasPrivate {
		if !storage.Exists(dataDir) {
			return fmt.Errorf("incomplete private peer state")
		}
		return storage.Save(dataDir, chain, pos, wallets)
	}
	// Preserve the legacy private-node behavior for existing P2P tests and
	// first-time private node state saves. Never let an observer lab directory
	// silently fall back to the private-wallet writer.
	if strings.Contains(strings.ToLower(filepath.Clean(dataDir)), "prism-v061-publicpeers") {
		return fmt.Errorf("public observer metadata missing: refusing private-wallet fallback")
	}
	return storage.Save(dataDir, chain, pos, wallets)
}

func regularFileExistsV061(path string) (bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("refusing non-regular state file: %s", path)
	}
	return true, nil
}
