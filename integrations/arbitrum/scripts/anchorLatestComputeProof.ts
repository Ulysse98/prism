import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { network } from "hardhat";
import {
  getAddress,
  zeroHash,
  type Address,
  type Hex,
} from "viem";

type ComputeJob = {
  id: string;
  status: string;
  worker: string;
  proofId: string;
};

type PrismProof = {
  id: string;
  proof_version?: number;
  job_id?: string;
  chain_id?: string;
  genesis_hash?: string;
  worker: string;
  task: {
    type: string;
  };
};

type PrismBlock = {
  height: number;
  useful_work?: PrismProof[];
};

type PrismChainFile = {
  blockchain: {
    Blocks: PrismBlock[];
  };
};

type PrismStatus = {
  chainId: string;
  genesisHash: string;
};

const prismApi =
  process.env.PRISM_API_URL ??
  "http://127.0.0.1:8080/api/v1";

function bytes32(value: string, label: string): Hex {
  const clean =
    value.startsWith("0x")
      ? value.slice(2)
      : value;

  if (!/^[0-9a-fA-F]{64}$/.test(clean)) {
    throw new Error(
      `${label} must contain exactly 64 hexadecimal characters`,
    );
  }

  return `0x${clean.toLowerCase()}` as Hex;
}

const statusResponse =
  await fetch(`${prismApi}/status`);

if (!statusResponse.ok) {
  throw new Error(
    `Prism status API returned HTTP ${statusResponse.status}`,
  );
}

const status =
  (await statusResponse.json()) as PrismStatus;

const jobsResponse =
  await fetch(`${prismApi}/compute/jobs`);

if (!jobsResponse.ok) {
  throw new Error(
    `Prism compute API returned HTTP ${jobsResponse.status}`,
  );
}

const jobsPayload =
  (await jobsResponse.json()) as {
    jobs: ComputeJob[];
  };

const jobsById =
  new Map(
    jobsPayload.jobs
      .filter((job) => job.status === "VERIFIED")
      .map((job) => [job.id, job]),
  );

const chainFile =
  new URL(
    "../../../data/chain.json",
    import.meta.url,
  );

const state =
  JSON.parse(
    readFileSync(chainFile, "utf8"),
  ) as PrismChainFile;

const matches: {
  block: number;
  proof: PrismProof;
  job: ComputeJob;
}[] = [];

for (const block of state.blockchain.Blocks) {
  for (const proof of block.useful_work ?? []) {
    if (proof.proof_version !== 2) {
      continue;
    }

    if (!proof.job_id) {
      continue;
    }

    const job =
      jobsById.get(proof.job_id);

    if (!job) {
      continue;
    }

    if (job.proofId !== proof.id) {
      continue;
    }

    if (job.worker !== proof.worker) {
      continue;
    }

    if (proof.chain_id !== status.chainId) {
      continue;
    }

    if (proof.genesis_hash !== status.genesisHash) {
      continue;
    }

    matches.push({
      block: block.height,
      proof,
      job,
    });
  }
}

matches.sort(
  (a, b) => b.block - a.block,
);

const latest =
  matches[0];

if (!latest) {
  throw new Error(
    "No verified Prism Proof v2 matches the compute marketplace",
  );
}

const jobId =
  bytes32(
    latest.job.id,
    "Job ID",
  );

const proofId =
  bytes32(
    latest.proof.id,
    "Proof ID",
  );

const workerIdHash =
  `0x${createHash("sha256")
    .update(latest.proof.worker, "utf8")
    .digest("hex")}` as Hex;

console.log();
console.log("LATEST VERIFIED PRISM COMPUTE PROOF");
console.log("-----------------------------------");
console.log("Block          :", latest.block);
console.log("Task           :", latest.proof.task.type);
console.log("Job ID         :", jobId);
console.log("Proof ID       :", proofId);
console.log("Worker         :", latest.proof.worker);
console.log("Worker ID hash :", workerIdHash);
console.log("Prism chain ID :", latest.proof.chain_id);
console.log();

const { viem } =
  await network.getOrCreate();

const publicClient =
  await viem.getPublicClient();

const wallets =
  await viem.getWalletClients();

if (wallets.length === 0) {
  throw new Error(
    "No Arbitrum recorder wallet configured",
  );
}

const recorder =
  wallets[0];

const chainId =
  await publicClient.getChainId();

const deploymentFile =
  new URL(
    `../ignition/deployments/chain-${chainId}/deployed_addresses.json`,
    import.meta.url,
  );

const deployments =
  JSON.parse(
    readFileSync(
      deploymentFile,
      "utf8",
    ),
  ) as Record<string, string>;

const key =
  "PrismProofRegistryModule#PrismProofRegistry";

const deployedAddress =
  deployments[key];

if (!deployedAddress) {
  throw new Error(
    `PrismProofRegistry deployment not found for chain ${chainId}`,
  );
}

const registryAddress =
  getAddress(
    deployedAddress,
  ) as Address;

const registry =
  await viem.getContractAt(
    "PrismProofRegistry",
    registryAddress,
    {
      client: {
        public: publicClient,
        wallet: recorder,
      },
    },
  );

const isRecorder =
  await registry.read.recorders([
    recorder.account.address,
  ]);

if (!isRecorder) {
  throw new Error(
    "Configured wallet is not an authorized Prism recorder",
  );
}

console.log("ARBITRUM TARGET");
console.log("----------------");
console.log("Chain ID :", chainId);
console.log("Registry :", registryAddress);
console.log("Recorder :", recorder.account.address);
console.log();

const existingJob =
  await registry.read.getJobForProof([
    proofId,
  ]);

const jobAlreadyAnchored =
  await registry.read.hasProof([
    jobId,
  ]);

if (
  existingJob !== zeroHash &&
  existingJob.toLowerCase() !== jobId.toLowerCase()
) {
  throw new Error(
    "Proof ID is already registered to another job",
  );
}

if (jobAlreadyAnchored) {
  console.log(
    "Proof already anchored — no transaction sent.",
  );
} else {
  console.log(
    "Registering Prism Proof v2 on Arbitrum Sepolia...",
  );

  const txHash =
    await registry.write.registerProof([
      jobId,
      proofId,
      workerIdHash,
    ]);

  console.log("TX :", txHash);
  console.log("Waiting for confirmation...");

  const receipt =
    await publicClient.waitForTransactionReceipt({
      hash: txHash,
    });

  console.log(
    "Block  :",
    receipt.blockNumber.toString(),
  );

  console.log(
    "Status :",
    receipt.status,
  );
}

const record =
  await registry.read.getProof([
    jobId,
  ]);

const proofCount =
  await registry.read.proofCount();

console.log();
console.log("ON-CHAIN PRISM PROOF");
console.log("--------------------");
console.log("Job ID         :", jobId);
console.log("Proof ID       :", record.proofId);
console.log("Worker ID hash :", record.workerIdHash);
console.log("Submitter      :", record.submitter);
console.log("Registered at  :", record.registeredAt.toString());
console.log("Exists         :", record.exists);
console.log("Proof count    :", proofCount.toString());
console.log();
console.log("✅ PRISM PROOF v2 / ARBITRUM ANCHORED");
