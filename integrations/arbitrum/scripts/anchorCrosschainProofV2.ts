import { network } from "hardhat";
import {
  getAddress,
  keccak256,
  stringToHex,
  type Address,
} from "viem";

const registryAddress = getAddress(
  process.env.PRISM_ARBITRUM_REGISTRY ??
    "0x44d872e47Aaf7874fc8Cf7236683f3E2548A2459",
) as Address;

const jobId =
  keccak256(stringToHex("prism-job-001"));

const proofId =
  keccak256(stringToHex("prism-proof-v2-001"));

const workerIdHash =
  keccak256(stringToHex("prism-worker-alice"));

const prismChainIdHash =
  keccak256(
    stringToHex("prism-d8c1f3e740b48957"),
  );

const { viem } = await network.getOrCreate();

const publicClient =
  await viem.getPublicClient();

const wallets =
  await viem.getWalletClients();

if (wallets.length === 0) {
  throw new Error("No wallet configured");
}

const wallet = wallets[0];

const registry =
  await viem.getContractAt(
    "PrismProofRegistry",
    registryAddress,
    {
      client: {
        public: publicClient,
        wallet,
      },
    },
  );

console.log();
console.log("PRISM PROOF V2 — ARBITRUM ANCHOR");
console.log("--------------------------------");
console.log("Registry :", registryAddress);
console.log("Recorder :", wallet.account.address);
console.log("Job ID   :", jobId);
console.log("Proof ID :", proofId);
console.log();

const owner =
  await registry.read.owner();

const allowed =
  await registry.read.recorders([
    wallet.account.address,
  ]);

console.log("Owner    :", owner);
console.log("Allowed  :", allowed);

if (!allowed) {
  throw new Error(
    "Configured wallet is not an authorized Prism recorder",
  );
}

const exists =
  await registry.read.hasProof([jobId]);

if (!exists) {
  console.log();
  console.log("Registering Proof v2...");

  const txHash =
    await registry.write.registerProofV2([
      jobId,
      proofId,
      workerIdHash,
      prismChainIdHash,
    ]);

  console.log("TX       :", txHash);

  const receipt =
    await publicClient.waitForTransactionReceipt({
      hash: txHash,
    });

  console.log(
    "Block    :",
    receipt.blockNumber.toString(),
  );
  console.log(
    "Status   :",
    receipt.status,
  );
} else {
  console.log();
  console.log(
    "Proof already registered — no transaction sent.",
  );
}

const registryId =
  await registry.read.getRegistryId([
    jobId,
  ]);

console.log();
console.log("Registry ID :", registryId);
console.log("✅ ARBITRUM PROOF V2 ANCHORED");
console.log();
