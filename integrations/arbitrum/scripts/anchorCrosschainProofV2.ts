import { network } from "hardhat";
import {
  getAddress,
  parseAbiItem,
  type Address,
  type Hex,
} from "viem";

import {
  loadCrossChainReceipt,
} from "./crosschainReceipt.js";

const prismApi =
  process.env.PRISM_API_URL?.replace(
    /\/$/,
    "",
  ) ??
  "http://127.0.0.1:8080/api/v1";

const registryAddress =
  getAddress(
    process.env.PRISM_ARBITRUM_REGISTRY ??
      "0x44d872e47Aaf7874fc8Cf7236683f3E2548A2459",
  ) as Address;

const receipt =
  loadCrossChainReceipt();

const jobId = receipt.jobId;
const proofId = receipt.proofId;
const workerIdHash =
  receipt.workerIdHash;
const prismChainIdHash =
  receipt.prismChainIdHash;

const { viem } =
  await network.getOrCreate();

const publicClient =
  await viem.getPublicClient();

const wallets =
  await viem.getWalletClients();

if (wallets.length === 0) {
  throw new Error(
    "No wallet configured",
  );
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
console.log(
  "PRISM PROOF V2 — ARBITRUM ANCHOR",
);
console.log(
  "--------------------------------",
);
console.log(
  "Registry :",
  registryAddress,
);
console.log(
  "Recorder :",
  wallet.account.address,
);
console.log(
  "Job ID   :",
  jobId,
);
console.log(
  "Proof ID :",
  proofId,
);
console.log();

const owner =
  await registry.read.owner();

const allowed =
  await registry.read.recorders([
    wallet.account.address,
  ]);

console.log(
  "Owner    :",
  owner,
);
console.log(
  "Allowed  :",
  allowed,
);

if (!allowed) {
  throw new Error(
    "Configured wallet is not an authorized Prism recorder",
  );
}

const exists =
  await registry.read.hasProof([
    jobId,
  ]);

let confirmedTxHash:
  Hex | null =
    null;

let confirmedBlockNumber:
  bigint | null =
    null;

if (!exists) {
  console.log();
  console.log(
    "Registering Proof v2...",
  );

  const txHash =
    await registry.write.registerProofV2([
      jobId,
      proofId,
      workerIdHash,
      prismChainIdHash,
    ]);

  console.log(
    "TX       :",
    txHash,
  );

  const txReceipt =
    await publicClient
      .waitForTransactionReceipt({
        hash: txHash,
      });

  console.log(
    "Block    :",
    txReceipt.blockNumber.toString(),
  );

  console.log(
    "Status   :",
    txReceipt.status,
  );

  if (
    txReceipt.status !==
    "success"
  ) {
    throw new Error(
      `Arbitrum transaction failed: ${txHash}`,
    );
  }

  confirmedTxHash =
    txHash;

  confirmedBlockNumber =
    txReceipt.blockNumber;
} else {
  console.log();

  console.log(
    "Proof already registered — recovering anchor transaction.",
  );

  const anchorEvent =
    parseAbiItem(
      "event ProofAnchorRegistered(bytes32 indexed registryId, bytes32 indexed jobId, bytes32 indexed proofId, bytes32 prismChainIdHash)",
    );

  const logs =
    await publicClient.getLogs({
      address: registryAddress,
      event: anchorEvent,
      args: {
        registryId:
          receipt.registryId,
        jobId,
        proofId,
      },
      fromBlock: 0n,
      toBlock: "latest",
    });

  const recovered =
    logs.find(
      (log) =>
        log.args.prismChainIdHash
          ?.toLowerCase() ===
        prismChainIdHash.toLowerCase(),
    );

  if (
    !recovered ||
    recovered.transactionHash === null ||
    recovered.blockNumber === null
  ) {
    throw new Error(
      "Cannot recover verified Arbitrum ProofAnchorRegistered event",
    );
  }

  confirmedTxHash =
    recovered.transactionHash;

  confirmedBlockNumber =
    recovered.blockNumber;

  console.log(
    "Recovered TX    :",
    confirmedTxHash,
  );

  console.log(
    "Recovered block :",
    confirmedBlockNumber.toString(),
  );
}

const registryId =
  await registry.read.getRegistryId([
    jobId,
  ]);

if (
  registryId.toLowerCase() !==
  receipt.registryId.toLowerCase()
) {
  throw new Error(
    `Arbitrum registry ID mismatch: got ${registryId}, expected ${receipt.registryId}`,
  );
}

console.log();
console.log(
  "Registry ID :",
  registryId,
);

if (
  confirmedTxHash !== null &&
  confirmedBlockNumber !== null
) {
  const settlementResponse =
    await fetch(
      `${prismApi}/settlements`,
      {
        method: "POST",
        headers: {
          "Content-Type":
            "application/json",
        },
        body: JSON.stringify({
          registryId,
          chain: "arbitrum",
          status: "confirmed",
          txHash:
            confirmedTxHash,
          registryAddress,
          blockNumber:
            Number(
              confirmedBlockNumber,
            ),
          explorerUrl:
            `https://sepolia.arbiscan.io/tx/${confirmedTxHash}`,
        }),
      },
    );

  if (
    !settlementResponse.ok
  ) {
    const body =
      await settlementResponse.text();

    throw new Error(
      `Prism settlement API returned HTTP ${settlementResponse.status}: ${body}`,
    );
  }

  console.log(
    "Prism settlement status: CONFIRMED",
  );
} else {
  console.log(
    "Settlement metadata not published: proof was already anchored.",
  );
}

console.log(
  "✅ ARBITRUM PROOF V2 ANCHORED",
);
console.log();
