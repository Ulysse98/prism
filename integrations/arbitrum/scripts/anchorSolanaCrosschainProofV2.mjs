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

const PROGRAM_ID = new PublicKey(
  "2yjpnNnDRnK4pvyRWLLMftAH2jfuAC2TnjyArVW3bhPz",
);

const jobId =
  "0xdbab0c89f752689748f4b1d375cd94800b40a1ed299e028edfa2336c0ba9a6cd";

const proofId =
  "0x42b5df2e467b316cd4fe45bdc303c15494854b917d8b98ee82402d1ef9076c38";

const workerIdHash =
  "0x9ef8a0bd338cccf1544c7c24b01e5149c939e254fa9c22bfc0c9a6fde7496726";

const prismChainIdHash =
  "0x27a955f0f028e1b3f397d20c36a77c7b8546fe31dd754bcf1fbdaae3ad95aedb";

const canonicalRegistryId =
  keccak256(
    concatHex([
      jobId,
      prismChainIdHash,
      proofId,
    ]),
  );

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
    "Proof TX     :",
    signature,
  );
} else {
  console.log();
  console.log(
    "Proof already registered ✅",
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

console.log(
  "✅ SOLANA PROOF V2 ANCHORED",
);
console.log(
  "✅ CANONICAL REGISTRY ID MATCH",
);
console.log();
