import { readFileSync } from "node:fs";

import {
  concatHex,
  keccak256,
  type Hex,
} from "viem";

export type CrossChainReceipt = {
  version: number;
  jobId: Hex;
  proofId: Hex;
  workerIdHash: Hex;
  prismChainIdHash: Hex;
  registryId: Hex;
};

const fallbackReceipt: CrossChainReceipt = {
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

function requireHex32(
  name: string,
  value: unknown,
): Hex {
  if (
    typeof value !== "string" ||
    !/^0x[0-9a-fA-F]{64}$/.test(value)
  ) {
    throw new Error(
      `${name} must be a 32-byte 0x-prefixed hex value`,
    );
  }

  return value as Hex;
}

function parseReceipt(
  value: unknown,
): CrossChainReceipt {
  if (
    value === null ||
    typeof value !== "object"
  ) {
    throw new Error(
      "cross-chain receipt JSON must be an object",
    );
  }

  const root =
    value as Record<string, unknown>;

  const candidate =
    root.crossChainReceipt &&
    typeof root.crossChainReceipt === "object"
      ? root.crossChainReceipt as Record<string, unknown>
      : root;

  const version =
    candidate.version;

  if (
    typeof version !== "number" ||
    version !== 1
  ) {
    throw new Error(
      `unsupported cross-chain receipt version: ${String(version)}`,
    );
  }

  const receipt: CrossChainReceipt = {
    version,
    jobId:
      requireHex32(
        "jobId",
        candidate.jobId,
      ),
    proofId:
      requireHex32(
        "proofId",
        candidate.proofId,
      ),
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

export function loadCrossChainReceipt():
CrossChainReceipt {
  const path =
    process.env.PRISM_CROSSCHAIN_RECEIPT_FILE?.trim();

  if (!path) {
    return fallbackReceipt;
  }

  return parseReceipt(
    JSON.parse(
      readFileSync(
        path,
        "utf8",
      ),
    ),
  );
}
