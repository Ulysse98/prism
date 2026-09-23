# Prism × Arbitrum Proof Registry

Prism anchors verified Proof v2 compute jobs on Arbitrum Sepolia.

The integration connects Prism's decentralized compute marketplace with an EVM registry that records a permanent link between:

- a Prism compute Job ID
- its verified Proof v2 ID
- the Prism worker identity hash
- the authorized recorder that submitted the anchor
- the Arbitrum registration timestamp

## Architecture

```text
Requester
   |
   v
Prism Compute Marketplace
   |
   | funded compute job
   v
Prism Worker
   |
   | executes useful work
   | signs Proof v2 with Ed25519
   v
Prism Verification
   |
   | verifies job context
   | verifies result
   | verifies worker identity
   | verifies signature
   v
Authorized Recorder
   |
   | registerProof(jobId, proofId, workerIdHash)
   v
PrismProofRegistry
   |
   v
Arbitrum Sepolia

```

## Deployed Contract

Network:

```text
Arbitrum Sepolia
Chain ID: 421614
```

PrismProofRegistry:

```text
0x8B4Cc27E3ACaF6b6deBC2c96462240eC196EfBC0
```

## Verified End-to-End Demo

A real Prism v0.40 Proof v2 has been successfully anchored on Arbitrum Sepolia.

Prism block:

```text
56
```

Task:

```text
ml_inference_quantized
```

Job ID:

```text
e09d1ea31bc3438f8475b713251bc0c6bd3b7477805b3e7b0d3004ac2266f6ca
```

Proof ID:

```text
64fcae59a11bd43192533a99a12a30098d2f3b4cf434bce3898a4cb3d06152e9
```

Worker:

```text
prism_97f844d6b007c3f940a57ef946959cfd38fecf2c
```

Worker identity hash:

```text
24c42023c68e4a15cb7b0fa17aab481c9de1d9a2a929167e2f089ba60f9c64e0
```

Arbitrum transaction:

```text
0x8e377cec1097e3d7d23a29f8b67c85ed9759b82fe1e72f720da7a245ba6f5b42
```

The transaction completed successfully and the registry returned:

```text
Exists      : true
Proof count : 1
```

## Contract Model

`PrismProofRegistry` stores one record per Prism compute job.

```solidity
struct ProofRecord {
    bytes32 proofId;
    bytes32 workerIdHash;
    address submitter;
    uint64 registeredAt;
    bool exists;
}
```

A proof is registered with:

```solidity
registerProof(
    bytes32 jobId,
    bytes32 proofId,
    bytes32 workerIdHash
)
```

The registry prevents:

- duplicate Job IDs
- duplicate Proof IDs
- zero Job IDs
- zero Proof IDs
- zero worker identity hashes
- submissions from unauthorized recorders

## Trust Model

The EVM contract does not reproduce Prism computation or Ed25519 verification.

Verification happens inside Prism before anchoring.

A Prism Proof v2 cryptographically binds:

```text
job ID
chain ID
genesis hash
task ID
worker
public key
result
output hash
score
```

The proof is signed by the Prism worker using Ed25519.

After Prism verifies the Proof v2 and the compute marketplace marks the job as `VERIFIED`, an authorized recorder anchors its identity on Arbitrum.

The recorder is therefore the current bridge trust boundary.

Future versions can reduce this trust assumption with stronger cross-chain verification or zero-knowledge proof verification.

## Run the Demo

### 1. Install the Arbitrum integration

```shell
cd integrations/arbitrum
npm ci
```

### 2. Configure the Arbitrum Sepolia recorder

```shell
npx hardhat keystore set ARBITRUM_SEPOLIA_PRIVATE_KEY
```

Do not commit private keys or `.env` files.

### 3. Check the network and recorder

```shell
npx hardhat run scripts/checkArbitrumSepolia.ts --network arbitrumSepolia
```

### 4. Start the Prism API

From the Prism repository root:

```shell
go run ./cmd/prism api --data data --host 127.0.0.1 --port 8080
```

### 5. Anchor the latest verified Proof v2

From `integrations/arbitrum`:

```shell
npx hardhat run scripts/anchorLatestComputeProof.ts --network arbitrumSepolia
```

The script:

1. reads the Prism compute marketplace
2. selects `VERIFIED` jobs
3. matches them against Proof v2 records in the Prism chain
4. validates Job ID, Proof ID, worker, chain ID, and genesis hash
5. hashes the Prism worker identity
6. checks recorder authorization
7. submits `registerProof(...)`
8. waits for Arbitrum confirmation
9. reads the record back from the contract

Running it again for an already anchored job does not submit a duplicate transaction.

### 6. Check the deployed registry

```shell
npx hardhat run scripts/checkPrismProofRegistry.ts --network arbitrumSepolia
```

Example:

```text
PRISM PROOF REGISTRY
--------------------
Chain ID    : 421614
Registry    : 0x8B4Cc27E3ACaF6b6deBC2c96462240eC196EfBC0
Is recorder : true
Proof count : 1
```

## Tests

```shell
npx hardhat compile
npx hardhat test solidity
```

Current result:

```text
18 passing
```

## Repository Layout

```text
contracts/
  PrismProofRegistry.sol
  PrismProofRegistry.t.sol

ignition/
  modules/
    PrismProofRegistry.ts
  deployments/
    chain-421614/

scripts/
  checkArbitrumSepolia.ts
  checkPrismProofRegistry.ts
  anchorLatestComputeProof.ts
```

## Status

```text
Prism v0.40
Proof v2
Funded Compute Marketplace
Arbitrum Sepolia
End-to-end anchor confirmed
```

The current integration demonstrates a complete Prism compute proof lifecycle from useful computation to a publicly queryable EVM anchor.

## Arbitrum Sepolia deployment

Prism Proof Registry V2 is deployed and live on Arbitrum Sepolia.

- Network: Arbitrum Sepolia
- Chain ID: `421614`
- Contract: `0x44d872e47Aaf7874fc8Cf7236683f3E2548A2459`
- Deployer: `0xcD24214E42927D9D97396d62CA4f75178c09acfe`
- Deployment transaction: `0x622a12112512fbcf1d5eeb82295d1047da01022417421a27ddd499d9278cec27`
- First Proof V2 transaction: `0x11140165432dd921ea3669042f0016f4812c92a39f437c3c3678d9c3dbc71921`

### First public Prism Proof V2

- Job ID: `0x683aa29701c286298ecdcab90f89677665598563157dda752da1f24994ca0e08`
- Proof ID: `0x5f725fb3054e7d67257f6138a0ea024a3e115f4251df0d9b9ea27a1a60526e22`
- Registry ID: `0x277416b6244ddc0104c6e5479034f6c2473d0c463dd3d377014a83780fb52a0e`
- Prism chain hash: `0x27a955f0f028e1b3f397d20c36a77c7b8546fe31dd754bcf1fbdaae3ad95aedb`

The proof was successfully registered with `registerProofV2` and independently read back from Arbitrum Sepolia.

### Verify from the command line

```bash
cast call \
  0x44d872e47Aaf7874fc8Cf7236683f3E2548A2459 \
  "proofCount()(uint256)" \
  --rpc-url https://sepolia-rollup.arbitrum.io/rpc
```

Expected result:

```text
2
```

## Cross-chain Proof V2 verification

Prism Proof V2 is now independently anchored and verified on both Arbitrum Sepolia and Solana Devnet.

### Canonical Proof V2

- Job ID: `0xdbab0c89f752689748f4b1d375cd94800b40a1ed299e028edfa2336c0ba9a6cd`
- Proof ID: `0x42b5df2e467b316cd4fe45bdc303c15494854b917d8b98ee82402d1ef9076c38`
- Prism chain hash: `0x27a955f0f028e1b3f397d20c36a77c7b8546fe31dd754bcf1fbdaae3ad95aedb`
- Canonical Registry ID: `0x0d2f7411a6f9e0263209bcc7278d172346e0b5916902bb22efc8c0ffbb1a618e`

### Arbitrum Sepolia

- Registry: `0x44d872e47Aaf7874fc8Cf7236683f3E2548A2459`
- Proof transaction: `0x945a6aff2945fdc93cbdea876e43e685e1e269b82846d32c8604b305e8e8b10f`

### Solana Devnet

- Program ID: `2yjpnNnDRnK4pvyRWLLMftAH2jfuAC2TnjyArVW3bhPz`
- Proof PDA: `8f4CgN5LNArQ1ZUYfQu8WtQJS2xD8dvccHrBmcsaeM6f`
- Proof transaction: `3zjDpkFV2EXtuWQgwcCqztEERTsZegToLC2qiFQmkCDhTArMwcDjxUZwTdS44b5hdkRyGC4HFLBaSkvutmyDBH9d`

### Verification

Run `npx hardhat run scripts/verifyCrosschainProofV2.ts --network arbitrumSepolia`.

Expected final result: `Arbitrum ↔ Solana ID : ✅ MATCH`.

`✅ PRISM PROOF V2 CROSS-CHAIN IDENTITY VERIFIED`

## Prism v0.41 real compute receipt E2E

Prism v0.41 was verified end to end with a receipt generated from a real funded compute settlement rather than a static test vector.

### Prism settlement

- Prism chain ID: `prism-7ab81e59be26f04c`
- Prism block: `1`
- Job ID: `0x199a9e9f49d131a63e29d274edd3f65c217f1fba17a8faeec678f3dc65415cab`
- Proof ID: `0x396ba3fc732f6eb8176806c84241126f525cb610f1b78590a79aec5193f4ed18`
- Worker identity hash: `0x1386a716cfa9f843b72f77d9fa78d927ed4919b053493719f6219a88933bde45`
- Prism chain ID hash: `0x72b18452d377a0c04435976c41641f5fba947981e2a1e9b9e24971508a7d3061`
- Settlement transaction: `e83d38806dde45eb3b298db47e998e22ab50024a26894ffbabf60ffb4d8134ee`
- Canonical Registry ID: `0xdf58bc5a9b24150c7e4b6ffa74d7af084be25692fd1a74da4ba6dff546650972`

### Arbitrum Sepolia

- Registry: `0x44d872e47Aaf7874fc8Cf7236683f3E2548A2459`
- Block: `311867093`
- Transaction: `0xc9f4fc89055c0d5acd0e6643ca9804b5a60d007ffb135588a8a723cf2eeb252c`
- Registry ID: `0xdf58bc5a9b24150c7e4b6ffa74d7af084be25692fd1a74da4ba6dff546650972`
- Reverse registry index: verified

### Solana Devnet

- Program ID: `2yjpnNnDRnK4pvyRWLLMftAH2jfuAC2TnjyArVW3bhPz`
- Proof PDA: `CZ4doHUfZJaYJSRoFjXUj427m7NkfG78w7fGutH8PhTn`
- Transaction: `2BJZsWjTJebkLZHsNrpQaNj6zsDfAkEaP9hT79LSZKH5dwJVZHXXbi4UZqtf7cA1KEqr3JtxUdFHfQLc9HB2d4Co`
- Registry ID: `0xdf58bc5a9b24150c7e4b6ffa74d7af084be25692fd1a74da4ba6dff546650972`
- Stored Proof v2 fields: verified

Final verifier result:

```text
Arbitrum ↔ Solana ID : ✅ MATCH

✅ PRISM PROOF V2 CROSS-CHAIN IDENTITY VERIFIED
```

This demonstrates the v0.41 pipeline:

```text
Prism compute job
        ↓
signed Proof v2
        ↓
Prism settlement
        ↓
crossChainReceipt
        ↓
Arbitrum Sepolia + Solana Devnet
        ↓
same canonical Registry ID
```
