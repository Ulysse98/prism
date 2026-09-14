import { readFileSync } from "node:fs";
import { network } from "hardhat";
import {
  getAddress,
  type Address,
  type Hex,
} from "viem";

// ------------------------------------------------------------
// Prism API types
// ------------------------------------------------------------

type PrismWorkEntry = {
  block: number;
  worker: string;
  workerAddress: string;
  task: string;
  taskId: string;
  result: number;
  resultValues?: number[];
  score: number;
  reward: number;
  verified: boolean;
  outputHash: string;
  proofId: string;
  blockHash: string;
};

type PrismWorkResponse = {
  count: number;
  entries: PrismWorkEntry[];
};

// ------------------------------------------------------------
// Configuration
// ------------------------------------------------------------

const prismApiBase =
  process.env.PRISM_API_URL ??
  "http://127.0.0.1:8080/api/v1";

const workEndpoint =
  `${prismApiBase}/work`;

// ------------------------------------------------------------
// Fetch latest verified Prism PoUW proof
// ------------------------------------------------------------

console.log();
console.log("PRISM → ARBITRUM BRIDGE");
console.log("========================");
console.log("Prism API :", workEndpoint);
console.log();

const response =
  await fetch(workEndpoint);

if (!response.ok) {
  throw new Error(
    `Prism API returned HTTP ${response.status} ${response.statusText}`,
  );
}

const workLog =
  (await response.json()) as PrismWorkResponse;

if (
  !Array.isArray(workLog.entries) ||
  workLog.entries.length === 0
) {
  throw new Error(
    "Prism worklog contains no PoUW proofs",
  );
}

const latestProof =
  workLog.entries.find(
    (entry) => entry.verified === true,
  );

if (!latestProof) {
  throw new Error(
    "No verified Prism PoUW proof found",
  );
}

// ------------------------------------------------------------
// Validate Prism proof identity
// ------------------------------------------------------------

let proofIdRaw =
  latestProof.proofId.trim();

if (proofIdRaw.startsWith("0x")) {
  proofIdRaw = proofIdRaw.slice(2);
}

if (!/^[0-9a-fA-F]{64}$/.test(proofIdRaw)) {
  throw new Error(
    `Invalid Prism proof ID: ${latestProof.proofId}`,
  );
}

const proofId =
  `0x${proofIdRaw.toLowerCase()}` as Hex;

if (!latestProof.workerAddress) {
  throw new Error(
    "Verified proof has no Prism worker address",
  );
}

if (!latestProof.task) {
  throw new Error(
    "Verified proof has no task type",
  );
}

if (
  !Number.isSafeInteger(latestProof.score) ||
  latestProof.score < 0
) {
  throw new Error(
    `Invalid Prism score: ${latestProof.score}`,
  );
}

const score =
  BigInt(latestProof.score);

// ------------------------------------------------------------
// Display Prism-side provenance
// ------------------------------------------------------------

console.log("LATEST VERIFIED PRISM PROOF");
console.log("---------------------------");
console.log(
  "Prism block :",
  latestProof.block,
);
console.log(
  "Block hash  :",
  latestProof.blockHash,
);
console.log(
  "Worker      :",
  latestProof.worker,
);
console.log(
  "Address     :",
  latestProof.workerAddress,
);
console.log(
  "Task        :",
  latestProof.task,
);
console.log(
  "Task ID     :",
  latestProof.taskId,
);

if (
  latestProof.resultValues &&
  latestProof.resultValues.length > 0
) {
  console.log(
    "Result      :",
    latestProof.resultValues,
  );
} else {
  console.log(
    "Result      :",
    latestProof.result,
  );
}

console.log(
  "Score       :",
  latestProof.score,
);
console.log(
  "Reward      :",
  latestProof.reward,
  "PRISM",
);
console.log(
  "Output hash :",
  latestProof.outputHash,
);
console.log(
  "Proof ID    :",
  proofId,
);
console.log(
  "Verified    :",
  latestProof.verified,
);
console.log();

// ------------------------------------------------------------
// Connect to current Hardhat network
// ------------------------------------------------------------

const { viem } =
  await network.getOrCreate();

const publicClient =
  await viem.getPublicClient();

const chainId =
  await publicClient.getChainId();

// ------------------------------------------------------------
// Load matching Ignition deployment
// ------------------------------------------------------------

const deploymentFile = new URL(
  `../ignition/deployments/chain-${chainId}/deployed_addresses.json`,
  import.meta.url,
);

const deployedAddresses =
  JSON.parse(
    readFileSync(
      deploymentFile,
      "utf8",
    ),
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
  getAddress(
    deployedAddress,
  ) as Address;

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

const recorder =
  wallets[0];

// ------------------------------------------------------------
// Contract instance
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
// Network display
// ------------------------------------------------------------

let networkName =
  `CHAIN ${chainId}`;

if (chainId === 31337) {
  networkName = "LOCAL EVM";
}

if (chainId === 421614) {
  networkName = "ARBITRUM SEPOLIA";
}

console.log("ANCHOR TARGET");
console.log("-------------");
console.log(
  "Network  :",
  networkName,
);
console.log(
  "Chain ID :",
  chainId,
);
console.log(
  "Registry :",
  registryAddress,
);
console.log(
  "Recorder :",
  recorder.account.address,
);
console.log();

// ------------------------------------------------------------
// Duplicate protection
// ------------------------------------------------------------

const alreadyAnchored =
  await registry.read.proofExists([
    proofId,
  ]);

if (alreadyAnchored) {
  console.log(
    "Proof already anchored — no transaction sent.",
  );
} else {
  console.log(
    "Anchoring Prism-verified PoUW proof...",
  );

  const txHash =
    await registry.write.recordUsefulWork([
      latestProof.workerAddress,
      proofId,
      latestProof.task,
      score,
    ]);

  console.log(
    "TX       :",
    txHash,
  );

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
// Read anchor back from EVM
// ------------------------------------------------------------

const anchoredProof =
  await registry.read.getProof([
    proofId,
  ]);

console.log();
console.log("ON-CHAIN ANCHOR");
console.log("----------------");
console.log(
  "Prism worker :",
  anchoredProof.prismWorker,
);
console.log(
  "Proof ID     :",
  anchoredProof.proofId,
);
console.log(
  "Work type    :",
  anchoredProof.workType,
);
console.log(
  "Score        :",
  anchoredProof.score.toString(),
);
console.log(
  "Anchored at  :",
  anchoredProof.anchoredAt.toString(),
);
console.log();

if (chainId === 421614) {
  console.log(
    "✅ PRISM VERIFIED / ARBITRUM ANCHORED",
  );
} else {
  console.log(
    "✅ PRISM VERIFIED / EVM ANCHORED",
  );
}