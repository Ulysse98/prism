package p2p

import (
	"crypto/ed25519"
	"fmt"
	"os"
	"path/filepath"

	"prism/internal/blockchain"
	"prism/internal/consensus"
	"prism/internal/storage"
	"prism/internal/wallet"
)

const (
	publicObserverMarkerV061      = ".prism-public-observer"
	publicObserverMarkerValueV061 = "prism-public-observer-v1\n"
)

// persistPeerStateV061 refuses to infer public/private wallet storage mode
// from the directory name. In particular, a keyless observer may NEVER fall
// back to storage.Save, which writes wallets.json.
//
// Existing public observers receive a durable marker on their next valid
// save. The marker makes public mode sticky even if api-wallets.json later
// disappears. Existing private nodes retain their ordinary storage format.
func persistPeerStateV061(
	dataDir string,
	chain *blockchain.Blockchain,
	pos *consensus.ProofOfStake,
	wallets map[string]*wallet.Wallet,
) error {
	privatePath := filepath.Join(dataDir, "wallets.json")
	publicPath := filepath.Join(dataDir, "api-wallets.json")
	chainPath := filepath.Join(dataDir, "chain.json")

	hasPrivate, err := regularFileExistsV061(privatePath)
	if err != nil {
		return err
	}
	hasPublic, err := regularFileExistsV061(publicPath)
	if err != nil {
		return err
	}
	markedPublic, err := inspectPublicObserverMarkerV061(dataDir)
	if err != nil {
		return err
	}

	if hasPrivate && hasPublic {
		return fmt.Errorf("ambiguous peer data: both public and private wallet files present")
	}
	if markedPublic && hasPrivate {
		return fmt.Errorf("public observer marker conflicts with private wallets.json")
	}

	if hasPublic || markedPublic {
		if !hasPublic || !storage.ExistsPublic(dataDir) {
			return fmt.Errorf("public observer metadata missing or incomplete: refusing private-wallet fallback")
		}
		if err := validatePeerWalletMaterialV061(wallets, false); err != nil {
			return err
		}
		if chain == nil || pos == nil || !chain.ValidateChain(pos) {
			return fmt.Errorf("refusing to persist invalid public observer chain")
		}
		// Set the sticky marker before the first public chain write: failure
		// leaves the observer in safe, non-private mode.
		if !markedPublic {
			if err := createPublicObserverMarkerV061(dataDir); err != nil {
				return err
			}
		}
		return storage.SavePublicChain(dataDir, chain, pos)
	}

	// No public metadata and no marker: only a complete in-memory private
	// wallet set can select private mode. Keyless or mixed wallet material
	// must fail, even in an arbitrarily named directory.
	if err := validatePeerWalletMaterialV061(wallets, true); err != nil {
		return err
	}
	if hasPrivate {
		if !storage.Exists(dataDir) {
			return fmt.Errorf("incomplete private peer state")
		}
		return storage.Save(dataDir, chain, pos, wallets)
	}

	// A brand-new private node can perform its first save. An existing
	// chain with missing wallet metadata cannot be silently reclassified.
	hasChain, err := regularFileExistsV061(chainPath)
	if err != nil {
		return err
	}
	if hasChain {
		return fmt.Errorf("peer wallet metadata missing: refusing storage mode fallback")
	}
	return storage.Save(dataDir, chain, pos, wallets)
}

func validatePeerWalletMaterialV061(wallets map[string]*wallet.Wallet, requirePrivate bool) error {
	if len(wallets) == 0 {
		return fmt.Errorf("peer wallet set is empty")
	}
	for name, w := range wallets {
		if w == nil || w.Address == "" {
			return fmt.Errorf("invalid peer wallet: %s", name)
		}
		if requirePrivate {
			if len(w.PrivateKey) != ed25519.PrivateKeySize {
				return fmt.Errorf("refusing private persistence without complete private key: %s", name)
			}
		} else if len(w.PrivateKey) != 0 {
			return fmt.Errorf("public observer contains private key material: %s", name)
		}
	}
	return nil
}

func inspectPublicObserverMarkerV061(dataDir string) (bool, error) {
	path := filepath.Join(dataDir, publicObserverMarkerV061)
	found, err := regularFileExistsV061(path)
	if err != nil || !found {
		return found, err
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read public observer marker: %w", err)
	}
	if string(contents) != publicObserverMarkerValueV061 {
		return false, fmt.Errorf("invalid public observer marker: refusing persistence")
	}
	return true, nil
}

func createPublicObserverMarkerV061(dataDir string) error {
	path := filepath.Join(dataDir, publicObserverMarkerV061)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		found, checkErr := inspectPublicObserverMarkerV061(dataDir)
		if checkErr != nil {
			return checkErr
		}
		if found {
			return nil
		}
		return fmt.Errorf("public observer marker creation conflicted")
	}
	if err != nil {
		return fmt.Errorf("create public observer marker: %w", err)
	}
	_, writeErr := f.WriteString(publicObserverMarkerValueV061)
	closeErr := f.Close()
	if writeErr != nil {
		return fmt.Errorf("write public observer marker: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close public observer marker: %w", closeErr)
	}
	return nil
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
