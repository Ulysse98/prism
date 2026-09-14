import { readFileSync } from "node:fs";
import { network } from "hardhat";
import {
  getAddress,
  type Address,
  type Hex,
} from "viem";

function env(name: string): string {
  const value = process.env[name];

  if (!value) {
    throw new Error(
      `Missing environment variable ${name}`,
    );
  }

  return value;
}

// ------------------------------------------------------------
// Prism proof data
// ------------------------------------------------------------

const prismWorker = env("PRISM_WORKER");
const workType = env("PRISM_WORK_TYPE");
const scoreRaw = env("PRISM_SCORE");

let proofIdRaw = env("PRISM_PROOF_ID");

if (proofIdRaw.startsWith("0x")) {
  proofIdRaw = proofIdRaw.slice(2);
}

if (!/^[0-9a-fA-F]{64}$/.test(proofIdRaw)) {
  throw new Error(
    "PRISM_PROOF_ID must contain exactly 64 hexadecimal characters",
  );
}

if (!/^\d+$/.test(scoreRaw)) {
  throw new Error(
    "PRISM_SCORE must be a positive integer",
  );
}

const proofId =
  `0x${proofIdRaw.toLowerCase()}` as Hex;

const score = BigInt(scoreRaw);

// ------------------------------------------------------------
// Hardhat / Viem network
// ------------------------------------------------------------

const { viem } =
  await network.getOrCreate();

const publicClient =
  await viem.getPublicClient();

const chainId =
  await publicClient.getChainId();

// ------------------------------------------------------------
// Load Ignition deployment for the current chain
//
// localhost          -> chain-31337
// Arbitrum Sepolia   -> chain-421614
// ------------------------------------------------------------

const deploymentFile = new URL(
  `../ignition/deployments/chain-${chainId}/deployed_addresses.json`,
  import.meta.url,
);

const deployedAddresses = JSON.parse(
  readFileSync(deploymentFile, "utf8"),
) as Record<string, string>;

const deploymentKey =
  "PrismWorkRegistryModule#PrismWorkRegistry";

const deployedAddress =
  deployedAddresses[deploymentKey];

if (!deployedAddress) {
  throw new Error(
    `PrismWorkRegistry deployment not found for chain ${chainId}`,
  );
}

const registryAddress =
  getAddress(deployedAddress) as Address;

// ------------------------------------------------------------
// Recorder wallet
// ------------------------------------------------------------

const wallets =
  await viem.getWalletClients();

if (wallets.length === 0) {
  throw new Error(
    "No recorder wallet configured for this network",
  );
}

const recorder = wallets[0];

// ------------------------------------------------------------
// Contract
// ------------------------------------------------------------

const registry =
  await viem.getContractAt(
    "PrismWorkRegistry",
    registryAddress,
    {
      client: {
        public: publicClient,
        wallet: recorder,
      },
    },
  );

// ------------------------------------------------------------
// Display context
// ------------------------------------------------------------

let networkName = `CHAIN ${chainId}`;

if (chainId === 31337) {
  networkName = "LOCAL EVM";
}

if (chainId === 421614) {
  networkName = "ARBITRUM SEPOLIA";
}

console.log();
console.log("PRISM × ARBITRUM ANCHOR");
console.log("-----------------------");
console.log("Network  :", networkName);
console.log("Chain ID :", chainId);
console.log("Registry :", registryAddress);
console.log(
  "Recorder :",
  recorder.account.address,
);
console.log("Worker   :", prismWorker);
console.log("Work     :", workType);
console.log("Score    :", score.toString());
console.log("Proof ID :", proofId);
console.log();

// ------------------------------------------------------------
// Prevent duplicate anchoring
// ------------------------------------------------------------

const exists =
  await registry.read.proofExists([
    proofId,
  ]);

if (exists) {
  console.log(
    "Proof already anchored — no transaction sent.",
  );
} else {
  console.log(
    "Anchoring verified Prism proof...",
  );

  const txHash =
    await registry.write.recordUsefulWork([
      prismWorker,
      proofId,
      workType,
      score,
    ]);

  console.log("TX       :", txHash);
  console.log(
    "Waiting for confirmation...",
  );

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
}

// ------------------------------------------------------------
// Read proof back from chain
// ------------------------------------------------------------

const proof =
  await registry.read.getProof([
    proofId,
  ]);

console.log();
console.log("ON-CHAIN ANCHOR");
console.log("----------------");
console.log(
  "Prism worker :",
  proof.prismWorker,
);
console.log(
  "Proof ID     :",
  proof.proofId,
);
console.log(
  "Work type    :",
  proof.workType,
);
console.log(
  "Score        :",
  proof.score.toString(),
);
console.log(
  "Anchored at  :",
  proof.anchoredAt.toString(),
);
console.log();

if (chainId === 421614) {
  console.log(
    "✅ PRISM VERIFIED / ARBITRUM ANCHORED",
  );
} else {
  console.log(
    "✅ VERIFIED PRISM PoUW PROOF ANCHORED",
  );
}