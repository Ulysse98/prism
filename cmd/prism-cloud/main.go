//go:build linux

// prism-cloud is a loopback-only, simulation-only SaaS gateway.
// It deliberately does not connect to wallet, GPU, escrow, or consensus APIs.
package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"prism/internal/computecloud"
)

const listenAddress = "127.0.0.1:18066"

type keyConfigV1 struct {
	Version int                          `json:"version"`
	Keys    []computecloud.CloudAPIKeyV1 `json:"keys"`
}

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)
	if len(os.Args) != 2 {
		log.Fatal("usage: prism-cloud init-dev | serve")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	conf := filepath.Join(home, ".config", "prism-cloud", "keys.json")
	data := filepath.Join(home, ".local", "share", "prism-cloud", "jobs.json")
	switch os.Args[1] {
	case "init-dev":
		if err := initDev(conf); err != nil {
			log.Fatal(err)
		}
	case "serve":
		if err := serve(conf, data); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatal("usage: prism-cloud init-dev | serve")
	}
}

func initDev(configPath string) error {
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	st, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !st.IsDir() || st.Mode().Perm()&0077 != 0 {
		return errors.New("config directory permissions must be 0700")
	}
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	token := "pc_dev_" + base64.RawURLEncoding.EncodeToString(random[:])
	hash := sha256.Sum256([]byte(token))
	cfg := keyConfigV1{Version: 1, Keys: []computecloud.CloudAPIKeyV1{{TenantID: "dev", SHA256: hex.EncodeToString(hash[:])}}}
	f, err := os.OpenFile(configPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("config already exists or cannot be created: %w", err)
	}
	if err := json.NewEncoder(f).Encode(cfg); err != nil {
		f.Close()
		os.Remove(configPath)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	fmt.Println("LOCAL DEVELOPMENT API KEY (shown once; keep private):")
	fmt.Println(token)
	fmt.Println("Config:", configPath)
	return nil
}

func serve(configPath, dataPath string) error {
	info, err := os.Lstat(configPath)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 8192 {
		return errors.New("API key config must be a small private (0600) regular file")
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	var cfg keyConfigV1
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return err
	}
	if cfg.Version != 1 {
		return errors.New("unsupported key configuration version")
	}
	store, err := computecloud.OpenLocalCloudStoreV1(dataPath)
	if err != nil {
		return err
	}
	defer store.Close()
	gateway := &computecloud.CloudGatewayV1{Store: store, Keys: cfg.Keys}
	if err := gateway.Validate(); err != nil {
		return err
	}
	observer, err := nodeObserverFromEnv()
	if err != nil {
		return err
	}
	observerHandler := &computecloud.CloudObserveGatewayV1{Gateway: gateway, Observer: observer}
	cloudHandler := &computecloud.CloudPlanGatewayV1{Observed: observerHandler}
	ln, err := net.Listen("tcp", listenAddress)
	if err != nil {
		return err
	}
	defer ln.Close()
	server := &http.Server{Handler: cloudHandler, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	log.Printf("Prism Cloud simulation listening on http://%s (LOCAL ONLY, NO COMPUTE)", listenAddress)
	return server.Serve(ln)
}

// nodeObserverFromEnv is opt-in and fails closed on partial or unsafe settings.
// The observer only reads a single loopback status endpoint.
func nodeObserverFromEnv() (*computecloud.CloudNodeObserverV1, error) {
	port := os.Getenv("PRISM_CLOUD_NODE_PORT")
	chainID := os.Getenv("PRISM_CLOUD_NODE_CHAIN_ID")
	genesis := os.Getenv("PRISM_CLOUD_NODE_GENESIS_HASH")
	if port == "" && chainID == "" && genesis == "" {
		return nil, nil
	}
	if port == "" || chainID == "" || genesis == "" {
		return nil, errors.New("node observer requires port, chain ID, and genesis hash")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil {
		return nil, errors.New("invalid node observer port")
	}
	return computecloud.NewCloudNodeObserverV1(portNumber, chainID, genesis)
}
