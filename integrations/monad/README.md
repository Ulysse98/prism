# Prism × Monad

Prism v0.55 supports canonical Proof v2 settlement on Monad Testnet.

## Network

- Network: Monad Testnet
- Chain ID: `10143`
- RPC: `https://testnet-rpc.monad.xyz`
- PrismProofRegistry: `0xc013BDeb0E20F73d613ed95FD3902802d73626d6`

## Settlement model

A Prism cross-chain receipt binds the compute Job ID, Proof ID,
worker identity hash and Prism chain identity hash to a deterministic
Registry ID.

The Monad integration anchors the Proof v2 receipt in PrismProofRegistry,
waits for transaction confirmation, validates the Registry ID and
publishes the confirmed settlement back to the Prism API.

## Security

Never commit the Monad deployment private key.
Use the Hardhat encrypted keystore with `MONAD_PRIVATE_KEY`.

Run Hardhat commands from `integrations/monad`.

## v0.55 E2E verification

- Job ID: `9725eff33668728a2c2fec9d9c34a74b680acdfd5b9af7ce1ae5b5263e9208c9`
- Proof ID: `5643ea58ed937f71b472344f4b743695e354c20ccf310d8bb40c5191a3bcec4b`
- Registry ID: `0xf367a0f5f7f480f347041dda02da926dd970bef456fc679d06e60ced901fade8`
- Monad transaction: `0x1c9d9eaba13bb2fceedb524443783292c6765cc2befec1c2837a0cc9625cde78`
- Monad block: `68444327`
- Prism settlement: `monad / verified`
