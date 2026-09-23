import { createHash } from "node:crypto";

import {
  createPublicClient,
  http,
  type Address,
  type Hex,
} from "viem";

import {
  loadCrossChainReceipt,
} from "./crosschainReceipt.js";

import { arbitrumSepolia } from "viem/chains";

import {
  Connection,
  PublicKey,
} from "@solana/web3.js";

/*
 * Prism Proof v2 cross-chain receipt.
 *
 * Set PRISM_CROSSCHAIN_RECEIPT_FILE to a real
 * /complete response JSON. Without it, the loader
 * keeps the historical canonical test vector as a
 * backwards-compatible demo fallback.
 */
const receipt =
  loadCrossChainReceipt();

const jobId = receipt.jobId;
const proofId = receipt.proofId;
const workerIdHash = receipt.workerIdHash;
const prismChainIdHash =
  receipt.prismChainIdHash;
const canonicalRegistryId =
  receipt.registryId;

/*
 * Arbitrum Sepolia.
 *
 * README currently documents this V2 deployment.
 * Override with PRISM_ARBITRUM_REGISTRY if needed.
 */
const arbitrumRegistry =
  (
    process.env.PRISM_ARBITRUM_REGISTRY ??
    "0x44d872e47Aaf7874fc8Cf7236683f3E2548A2459"
  ) as Address;

const arbitrumClient = createPublicClient({
  chain: arbitrumSepolia,
  transport: http(process.env.ARBITRUM_RPC_URL),
});

const registryAbi = [
  {
    type: "function",
    name: "hasProof",
    stateMutability: "view",
    inputs: [
      {
        name: "jobId",
        type: "bytes32",
      },
    ],
    outputs: [
      {
        name: "",
        type: "bool",
      },
    ],
  },
  {
    type: "function",
    name: "getRegistryId",
    stateMutability: "view",
    inputs: [
      {
        name: "jobId",
        type: "bytes32",
      },
    ],
    outputs: [
      {
        name: "",
        type: "bytes32",
      },
    ],
  },
  {
    type: "function",
    name: "getJobForRegistry",
    stateMutability: "view",
    inputs: [
      {
        name: "registryId",
        type: "bytes32",
      },
    ],
    outputs: [
      {
        name: "",
        type: "bytes32",
      },
    ],
  },
] as const;

/*
 * Solana Prism Proof Registry.
 */
const solanaProgramId = new PublicKey(
  process.env.PRISM_SOLANA_PROGRAM_ID ??
    "2yjpnNnDRnK4pvyRWLLMftAH2jfuAC2TnjyArVW3bhPz",
);

const solanaRpc =
  process.env.SOLANA_RPC_URL ??
  "https://api.devnet.solana.com";

const solana = new Connection(
  solanaRpc,
  "confirmed",
);

function hex(bytes: Uint8Array): Hex {
  return `0x${Buffer.from(bytes).toString("hex")}`;
}

function equalHex(a: string, b: string): boolean {
  return a.toLowerCase() === b.toLowerCase();
}

async function verifyArbitrum() {
  const code = await arbitrumClient.getCode({
    address: arbitrumRegistry,
  });

  if (!code || code === "0x") {
    return {
      deployed: false,
      registered: false,
      registryId: null as Hex | null,
      idMatches: false,
      reverseMatches: false,
    };
  }

  const registered =
    await arbitrumClient.readContract({
      address: arbitrumRegistry,
      abi: registryAbi,
      functionName: "hasProof",
      args: [jobId],
    });

  if (!registered) {
    return {
      deployed: true,
      registered: false,
      registryId: null as Hex | null,
      idMatches: false,
      reverseMatches: false,
    };
  }

  const registryId =
    await arbitrumClient.readContract({
      address: arbitrumRegistry,
      abi: registryAbi,
      functionName: "getRegistryId",
      args: [jobId],
    });

  const reverseJob =
    await arbitrumClient.readContract({
      address: arbitrumRegistry,
      abi: registryAbi,
      functionName: "getJobForRegistry",
      args: [registryId],
    });

  return {
    deployed: true,
    registered: true,
    registryId,
    idMatches:
      equalHex(
        registryId,
        canonicalRegistryId,
      ),
    reverseMatches:
      equalHex(reverseJob, jobId),
  };
}

async function verifySolana() {
  const program =
    await solana.getAccountInfo(
      solanaProgramId,
    );

  if (!program?.executable) {
    return {
      deployed: false,
      registered: false,
      proofPda: null,
      registryId: null as Hex | null,
      idMatches: false,
      fieldsMatch: false,
    };
  }

  const jobBytes =
    Buffer.from(
      jobId.slice(2),
      "hex",
    );

  const [proofPda] =
    PublicKey.findProgramAddressSync(
      [
        Buffer.from("job"),
        jobBytes,
      ],
      solanaProgramId,
    );

  const account =
    await solana.getAccountInfo(
      proofPda,
    );

  if (!account) {
    return {
      deployed: true,
      registered: false,
      proofPda:
        proofPda.toBase58(),
      registryId: null as Hex | null,
      idMatches: false,
      fieldsMatch: false,
    };
  }

  /*
   * Anchor ProofRecord:
   *
   *  0..8    discriminator
   *  8..40   job_id
   * 40..72   proof_id
   * 72..104  worker_id_hash
   * 104..136 prism_chain_id_hash
   * 136..168 registry_id
   */
  if (account.data.length < 168) {
    throw new Error(
      `Unexpected Solana ProofRecord size: ${account.data.length}`,
    );
  }

  const expectedDiscriminator =
    createHash("sha256")
      .update("account:ProofRecord")
      .digest()
      .subarray(0, 8);

  const discriminator =
    account.data.subarray(0, 8);

  if (
    !Buffer.from(discriminator).equals(
      expectedDiscriminator,
    )
  ) {
    throw new Error(
      "Solana account is not a ProofRecord",
    );
  }

  const storedJobId =
    hex(account.data.subarray(8, 40));

  const storedProofId =
    hex(account.data.subarray(40, 72));

  const storedWorkerId =
    hex(account.data.subarray(72, 104));

  const storedChainId =
    hex(account.data.subarray(104, 136));

  const storedRegistryId =
    hex(account.data.subarray(136, 168));

  const fieldsMatch =
    equalHex(storedJobId, jobId) &&
    equalHex(storedProofId, proofId) &&
    equalHex(
      storedWorkerId,
      workerIdHash,
    ) &&
    equalHex(
      storedChainId,
      prismChainIdHash,
    );

  return {
    deployed: true,
    registered: true,
    proofPda:
      proofPda.toBase58(),
    registryId:
      storedRegistryId,
    idMatches:
      equalHex(
        storedRegistryId,
        canonicalRegistryId,
      ),
    fieldsMatch,
  };
}

async function main() {
  console.log();
  console.log(
    "PRISM PROOF V2 — CROSS-CHAIN VERIFIER",
  );
  console.log(
    "======================================",
  );
  console.log();

  console.log(
    "Job ID       :",
    jobId,
  );
  console.log(
    "Proof ID     :",
    proofId,
  );
  console.log(
    "Worker hash  :",
    workerIdHash,
  );
  console.log(
    "Prism chain  :",
    prismChainIdHash,
  );

  console.log();

  console.log(
    "Canonical ID :",
    canonicalRegistryId,
  );

  console.log();

  /*
   * Arbitrum
   */
  console.log(
    "ARBITRUM SEPOLIA",
  );
  console.log(
    "----------------",
  );
  console.log(
    "Registry      :",
    arbitrumRegistry,
  );

  let arbitrum;

  try {
    arbitrum =
      await verifyArbitrum();

    console.log(
      "Contract      :",
      arbitrum.deployed
        ? "✅ DEPLOYED"
        : "❌ NOT DEPLOYED",
    );

    console.log(
      "Proof         :",
      arbitrum.registered
        ? "✅ REGISTERED"
        : "❌ NOT FOUND",
    );

    if (arbitrum.registryId) {
      console.log(
        "Registry ID   :",
        arbitrum.registryId,
      );
    }

    console.log(
      "Canonical ID  :",
      arbitrum.idMatches
        ? "✅ MATCH"
        : "❌ MISMATCH / ABSENT",
    );

    console.log(
      "Reverse index :",
      arbitrum.reverseMatches
        ? "✅ MATCH"
        : "❌ MISMATCH / ABSENT",
    );
  } catch (error) {
    console.error(
      "Arbitrum      : ❌ RPC / ABI ERROR",
    );
    console.error(error);

    arbitrum = {
      deployed: false,
      registered: false,
      registryId: null,
      idMatches: false,
      reverseMatches: false,
    };
  }

  console.log();

  /*
   * Solana
   */
  console.log(
    "SOLANA DEVNET",
  );
  console.log(
    "-------------",
  );
  console.log(
    "Program       :",
    solanaProgramId.toBase58(),
  );

  let solanaResult;

  try {
    solanaResult =
      await verifySolana();

    console.log(
      "Program       :",
      solanaResult.deployed
        ? "✅ DEPLOYED"
        : "❌ NOT DEPLOYED",
    );

    console.log(
      "Proof         :",
      solanaResult.registered
        ? "✅ REGISTERED"
        : "❌ NOT FOUND",
    );

    if (solanaResult.proofPda) {
      console.log(
        "Proof PDA     :",
        solanaResult.proofPda,
      );
    }

    if (solanaResult.registryId) {
      console.log(
        "Registry ID   :",
        solanaResult.registryId,
      );
    }

    console.log(
      "Proof fields  :",
      solanaResult.fieldsMatch
        ? "✅ MATCH"
        : "❌ MISMATCH / ABSENT",
    );

    console.log(
      "Canonical ID  :",
      solanaResult.idMatches
        ? "✅ MATCH"
        : "❌ MISMATCH / ABSENT",
    );
  } catch (error) {
    console.error(
      "Solana        : ❌ RPC / ACCOUNT ERROR",
    );
    console.error(error);

    solanaResult = {
      deployed: false,
      registered: false,
      proofPda: null,
      registryId: null,
      idMatches: false,
      fieldsMatch: false,
    };
  }

  console.log();

  const crossChainMatch =
    arbitrum.registryId !== null &&
    solanaResult.registryId !== null &&
    equalHex(
      arbitrum.registryId,
      solanaResult.registryId,
    );

  console.log(
    "CROSS-CHAIN",
  );
  console.log(
    "-----------",
  );

  console.log(
    "Arbitrum ↔ Solana ID :",
    crossChainMatch
      ? "✅ MATCH"
      : "❌ NOT VERIFIED",
  );

  const verified =
    arbitrum.deployed &&
    arbitrum.registered &&
    arbitrum.idMatches &&
    arbitrum.reverseMatches &&
    solanaResult.deployed &&
    solanaResult.registered &&
    solanaResult.fieldsMatch &&
    solanaResult.idMatches &&
    crossChainMatch;

  console.log();

  if (verified) {
    console.log(
      "✅ PRISM PROOF V2 CROSS-CHAIN IDENTITY VERIFIED",
    );
    console.log();
    return;
  }

  console.log(
    "⚠️ PRISM CROSS-CHAIN IDENTITY NOT YET FULLY VERIFIED",
  );
  console.log();

  process.exitCode = 1;
}

await main();
