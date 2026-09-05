# Prism v0.17 Devnet Runbook

Prism v0.17 provides a reproducible three-node Docker devnet with persistent state and periodic P2P synchronization.

## Verify

```powershell
go test ./...
docker ps --filter "name=prism-node"
```

## Build

```powershell
docker build -t prism-node:v0.17 .
```

## Start the devnet

```powershell
docker compose up -d
Start-Sleep -Seconds 8
```

The devnet contains:

- prism-node-1
- prism-node-2
- prism-node-3

All nodes should report:

```text
PRISM NODE v0.17
Protocol: 0.17
Chain state: MATCH
```

## Offline node recovery demo

Stop node 3:

```powershell
docker compose stop node-3
```

Stop node 1 and produce two new blocks directly in its persistent state:

```powershell
docker compose stop node-1

docker run --rm `
  -v prism-devnet-node1-data:/app/data `
  prism-node:v0.17 `
  node-produce --port 7001

docker run --rm `
  -v prism-devnet-node1-data:/app/data `
  prism-node:v0.17 `
  node-produce --port 7001
```

Restart node 1:

```powershell
docker compose start node-1
Start-Sleep -Seconds 8
```

Node 2 should detect that it is behind and automatically synchronize.

Expected output:

```text
Chain state: LOCAL NODE BEHIND
Requesting synchronization...
Remote blockchain validation: VALID
=== SYNCHRONIZATION COMPLETE ===
Chain valid: true
```

Restart stale node 3:

```powershell
docker compose start node-3
Start-Sleep -Seconds 8
docker logs --tail 60 prism-node-3
```

Node 3 should validate the remote blockchain, catch up, persist the new state, and eventually report:

```text
Chain state: MATCH
```

## Prism v0.17 demonstrated properties

- three Dockerized nodes
- persistent node state
- Chain ID and Genesis consistency
- P2P handshake validation
- periodic peer synchronization
- stale-node detection
- remote-chain validation before adoption
- automatic offline-node recovery
- Proof of Stake
- Proof of Useful Work
- humanity-gated Proof of Useful Participation

Prism v0.17 is an experimental blockchain protocol prototype built for ETHOnline 2026.
