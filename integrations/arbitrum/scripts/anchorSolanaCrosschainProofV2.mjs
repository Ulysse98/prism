import { readFileSync } from "node:fs";
import { homedir } from "node:os";
import { join } from "node:path";
import { createHash } from "node:crypto";

import {
  Connection,
  Keypair,
  PublicKey,
  SystemProgram,
  Transaction,
  TransactionInstruction,
  sendAndConfirmTransaction,
} from "@solana/web3.js";

import {
  concatHex,
  keccak256,
} from "viem";

const RPC = "https://api.devnet.solana.com";

const prismApi =
  process.env.PRISM_API_URL?.replace(
    /\/$/,
    "",
  ) ??
  "http://127.0.0.1:8080/api/v1";

const PROGRAM_ID = new PublicKey(
  "2yjpnNnDRnK4pvyRWLLMftAH2jfuAC2TnjyArVW3bhPz",
);

const fallbackReceipt = {
  version: 1,
  jobId:
    "0xdbab0c89f752689748f4b1d375cd94800b40a1ed299e028edfa2336c0ba9a6cd",
  proofId:
    "0x42b5df2e467b316cd4fe45bdc303c15494854b917d8b98ee82402d1ef9076c38",
  workerIdHash:
    "0x9ef8a0bd338cccf1544c7c24b01e5149c939e254fa9c22bfc0c9a6fde7496726",
  prismChainIdHash:
    "0x27a955f0f028e1b3f397d20c36a77c7b8546fe31dd754bcf1fbdaae3ad95aedb",
  registryId:
    "0x0d2f7411a6f9e0263209bcc7278d172346e0b5916902bb22efc8c0ffbb1a618e",
};

function requireHex32(name, value) {
  if (
    typeof value !== "string" ||
    !/^0x[0-9a-fA-F]{64}$/.test(value)
  ) {
    throw new Error(
      `${name} must be a 32-byte 0x-prefixed hex value`,
    );
  }

  return value;
}

function loadReceipt() {
  const receiptPath =
    process.env.PRISM_CROSSCHAIN_RECEIPT_FILE?.trim();

  if (!receiptPath) {
    return fallbackReceipt;
  }

  const root =
    JSON.parse(
      readFileSync(receiptPath, "utf8"),
    );

  const candidate =
    root?.crossChainReceipt ?? root;

  if (
    candidate === null ||
    typeof candidate !== "object"
  ) {
    throw new Error(
      "cross-chain receipt JSON must be an object",
    );
  }

  if (candidate.version !== 1) {
    throw new Error(
      `unsupported cross-chain receipt version: ${String(candidate.version)}`,
    );
  }

  const receipt = {
    version: candidate.version,
    jobId:
      requireHex32("jobId", candidate.jobId),
    proofId:
      requireHex32("proofId", candidate.proofId),
    workerIdHash:
      requireHex32(
        "workerIdHash",
        candidate.workerIdHash,
      ),
    prismChainIdHash:
      requireHex32(
        "prismChainIdHash",
        candidate.prismChainIdHash,
      ),
    registryId:
      requireHex32(
        "registryId",
        candidate.registryId,
      ),
  };

  const expectedRegistryId =
    keccak256(
      concatHex([
        receipt.jobId,
        receipt.prismChainIdHash,
        receipt.proofId,
      ]),
    );

  if (
    expectedRegistryId.toLowerCase() !==
    receipt.registryId.toLowerCase()
  ) {
    throw new Error(
      `cross-chain registryId mismatch: got ${receipt.registryId}, expected ${expectedRegistryId}`,
    );
  }

  return receipt;
}

const receipt = loadReceipt();

const jobId = receipt.jobId;
const proofId = receipt.proofId;
const workerIdHash = receipt.workerIdHash;
const prismChainIdHash =
  receipt.prismChainIdHash;
const canonicalRegistryId =
  receipt.registryId;

function bytes32(hex) {
  return Buffer.from(hex.slice(2), "hex");
}

function discriminator(name) {
  return createHash("sha256")
    .update(`global:${name}`)
    .digest()
    .subarray(0, 8);
}

function hex(bytes) {
  return `0x${Buffer.from(bytes).toString("hex")}`;
}

const walletPath =
  process.env.PRISM_SOLANA_KEYPAIR?.trim() ||
  join(
    homedir(),
    ".config/solana/id.json",
  );

const secret =
  JSON.parse(
    readFileSync(walletPath, "utf8"),
  );

const payer =
  Keypair.fromSecretKey(
    Uint8Array.from(secret),
  );

const connection =
  new Connection(RPC, "confirmed");

const jobBytes = bytes32(jobId);
const proofBytes = bytes32(proofId);

const [configPda] =
  PublicKey.findProgramAddressSync(
    [Buffer.from("registry")],
    PROGRAM_ID,
  );

const [proofPda] =
  PublicKey.findProgramAddressSync(
    [
      Buffer.from("job"),
      jobBytes,
    ],
    PROGRAM_ID,
  );

const [proofIndexPda] =
  PublicKey.findProgramAddressSync(
    [
      Buffer.from("proof"),
      proofBytes,
    ],
    PROGRAM_ID,
  );

console.log();
console.log("PRISM SOLANA PROOF V2 ANCHOR");
console.log("============================");
console.log();
console.log("RPC          :", RPC);
console.log("Program      :", PROGRAM_ID.toBase58());
console.log("Recorder     :", payer.publicKey.toBase58());
console.log("Config PDA   :", configPda.toBase58());
console.log("Proof PDA    :", proofPda.toBase58());
console.log("Proof index  :", proofIndexPda.toBase58());
console.log("Registry ID  :", canonicalRegistryId);
console.log();

//
// 1. Initialize registry
//

let configAccount =
  await connection.getAccountInfo(
    configPda,
  );

if (!configAccount) {
  console.log(
    "Initializing Prism registry...",
  );

  const ix =
    new TransactionInstruction({
      programId: PROGRAM_ID,
      keys: [
        {
          pubkey: configPda,
          isSigner: false,
          isWritable: true,
        },
        {
          pubkey: payer.publicKey,
          isSigner: true,
          isWritable: true,
        },
        {
          pubkey: SystemProgram.programId,
          isSigner: false,
          isWritable: false,
        },
      ],
      data: discriminator(
        "initialize_registry",
      ),
    });

  const signature =
    await sendAndConfirmTransaction(
      connection,
      new Transaction().add(ix),
      [payer],
      {
        commitment: "confirmed",
      },
    );

  console.log(
    "Initialize TX:",
    signature,
  );
} else {
  console.log(
    "Registry already initialized ✅",
  );
}

//
// 2. Register Proof v2
//

let confirmedSignature = null;
let confirmedSlot = null;

let proofAccount =
  await connection.getAccountInfo(
    proofPda,
  );

if (!proofAccount) {
  console.log();
  console.log(
    "Registering canonical Proof v2...",
  );

  const data =
    Buffer.concat([
      discriminator(
        "register_proof_v2",
      ),
      jobBytes,
      proofBytes,
      bytes32(workerIdHash),
      bytes32(prismChainIdHash),
    ]);

  const ix =
    new TransactionInstruction({
      programId: PROGRAM_ID,
      keys: [
        {
          pubkey: configPda,
          isSigner: false,
          isWritable: true,
        },
        {
          pubkey: proofPda,
          isSigner: false,
          isWritable: true,
        },
        {
          pubkey: proofIndexPda,
          isSigner: false,
          isWritable: true,
        },
        {
          pubkey: payer.publicKey,
          isSigner: true,
          isWritable: true,
        },
        {
          pubkey: SystemProgram.programId,
          isSigner: false,
          isWritable: false,
        },
      ],
      data,
    });

  confirmedSignature =
    await sendAndConfirmTransaction(
      connection,
      new Transaction().add(ix),
      [payer],
      {
        commitment: "confirmed",
      },
    );

  console.log(
    "Proof TX     :",
    confirmedSignature,
  );

  const statusResponse =
    await connection.getSignatureStatuses(
      [confirmedSignature],
      {
        searchTransactionHistory: true,
      },
    );

  const signatureStatus =
    statusResponse.value[0];

  if (
    !signatureStatus ||
    signatureStatus.err !== null
  ) {
    throw new Error(
      `Solana transaction was not confirmed successfully: ${confirmedSignature}`,
    );
  }

  confirmedSlot =
    signatureStatus.slot;

  console.log(
    "Slot         :",
    confirmedSlot,
  );
} else {
  console.log();
  console.log(
    "Proof already registered ✅",
  );

  const signatureHistory =
    await connection.getSignaturesForAddress(
      proofPda,
      {
        limit: 20,
      },
      "confirmed",
    );

  const recovered =
    signatureHistory.find(
      (entry) =>
        entry.err === null,
    );

  if (!recovered) {
    throw new Error(
      "Cannot recover confirmed Solana registration transaction for existing Proof v2",
    );
  }

  confirmedSignature =
    recovered.signature;
  confirmedSlot =
    recovered.slot;

  console.log(
    "Recovered TX  :",
    confirmedSignature,
  );
  console.log(
    "Recovered slot:",
    confirmedSlot,
  );
}

//
// 3. Read ProofRecord back
//

proofAccount =
  await connection.getAccountInfo(
    proofPda,
    "confirmed",
  );

if (!proofAccount) {
  throw new Error(
    "ProofRecord missing after registration",
  );
}

if (proofAccount.data.length < 168) {
  throw new Error(
    `Unexpected ProofRecord size: ${proofAccount.data.length}`,
  );
}

const storedJobId =
  hex(
    proofAccount.data.subarray(
      8,
      40,
    ),
  );

const storedProofId =
  hex(
    proofAccount.data.subarray(
      40,
      72,
    ),
  );

const storedWorker =
  hex(
    proofAccount.data.subarray(
      72,
      104,
    ),
  );

const storedChain =
  hex(
    proofAccount.data.subarray(
      104,
      136,
    ),
  );

const storedRegistry =
  hex(
    proofAccount.data.subarray(
      136,
      168,
    ),
  );

console.log();
console.log("ON-CHAIN PROOF RECORD");
console.log("---------------------");

console.log(
  "Job ID       :",
  storedJobId,
);

console.log(
  "Proof ID     :",
  storedProofId,
);

console.log(
  "Worker       :",
  storedWorker,
);

console.log(
  "Prism chain  :",
  storedChain,
);

console.log(
  "Registry ID  :",
  storedRegistry,
);

console.log();

if (
  storedJobId.toLowerCase() !==
    jobId.toLowerCase() ||
  storedProofId.toLowerCase() !==
    proofId.toLowerCase() ||
  storedWorker.toLowerCase() !==
    workerIdHash.toLowerCase() ||
  storedChain.toLowerCase() !==
    prismChainIdHash.toLowerCase() ||
  storedRegistry.toLowerCase() !==
    canonicalRegistryId.toLowerCase()
) {
  throw new Error(
    "❌ SOLANA PROOF V2 DATA MISMATCH",
  );
}

if (
  confirmedSignature !== null &&
  confirmedSlot !== null
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
          registryId:
            canonicalRegistryId,
          chain: "solana",
          status: "confirmed",
          txHash:
            confirmedSignature,
          registryAddress:
            PROGRAM_ID.toBase58(),
          blockNumber:
            confirmedSlot,
          explorerUrl:
            `https://explorer.solana.com/tx/${confirmedSignature}?cluster=devnet`,
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
  "✅ SOLANA PROOF V2 ANCHORED",
);
console.log(
  "✅ CANONICAL REGISTRY ID MATCH",
);
console.log();
