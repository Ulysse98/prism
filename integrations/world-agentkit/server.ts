import {
  buildAgentkitSchema,
  createAgentBookVerifier,
  parseAgentkitHeader,
  validateAgentkitMessage,
  verifyAgentkitSignature,
} from "@worldcoin/agentkit";

import { createHash, randomUUID } from "node:crypto";
import { createServer, type ServerResponse } from "node:http";

const HOST = "127.0.0.1";
const PORT = 4021;
const RESOURCE = `http://${HOST}:${PORT}/poup`;

const validNonces = new Map<string, number>();
const agentBook = createAgentBookVerifier();

function json(
  res: ServerResponse,
  status: number,
  body: unknown
) {
  res.writeHead(status, {
    "content-type": "application/json; charset=utf-8",
  });

  res.end(JSON.stringify(body, null, 2));
}

function createChallenge() {
  const now = new Date();

  const nonce = randomUUID().replaceAll("-", "");
  const expiration = Date.now() + 5 * 60_000;

  validNonces.set(nonce, expiration);

  return {
    info: {
      domain: HOST,
      uri: RESOURCE,
      resources: [RESOURCE],
      statement:
        "Authorize a human-backed agent for Prism Proof of Useful Participation",
      version: "1",
      nonce,
      issuedAt: now.toISOString(),
      expirationTime: new Date(expiration).toISOString(),
    },

    supportedChains: [
      {
        chainId: "eip155:480",
        type: "eip191" as const,
        signatureScheme: "eip191" as const,
      },
    ],

    schema: buildAgentkitSchema(),
  };
}

const server = createServer(async (req, res) => {
  if (req.method !== "GET" || req.url !== "/poup") {
    json(res, 404, {
      error: "not_found",
    });

    return;
  }

  const header = req.headers["agentkit"];

  // First request: issue AgentKit challenge.
  if (typeof header !== "string") {
    const extension = createChallenge();

    console.log();
    console.log("→ Agent requested /poup");
    console.log("← 402 AgentKit challenge issued");

    json(res, 402, {
      error: "agentkit_verification_required",

      extensions: {
        agentkit: extension,
      },
    });

    return;
  }

  console.log();
  console.log("→ Signed AgentKit request received");

  try {
    const payload = parseAgentkitHeader(header);

    const validation = await validateAgentkitMessage(
      payload,
      RESOURCE,
      {
        checkNonce: (nonce) => {
          const expiration = validNonces.get(nonce);

          return (
            expiration !== undefined &&
            expiration >= Date.now()
          );
        },
      }
    );

    if (!validation.valid) {
      console.log("AgentKit message: INVALID");

      json(res, 401, {
        authenticated: false,
        error: validation.error,
        poup: "DENIED",
      });

      return;
    }

    console.log("AgentKit message: VALID");

    const verification =
      await verifyAgentkitSignature(payload);

    if (
      !verification.valid ||
      !verification.address
    ) {
      console.log("Agent signature: INVALID");

      json(res, 401, {
        authenticated: false,
        error: verification.error,
        poup: "DENIED",
      });

      return;
    }

    // Consume nonce after valid signature.
    validNonces.delete(payload.nonce);

    console.log("Agent signature: VERIFIED");
    console.log(
      "Recovered agent:",
      verification.address
    );

    const humanId =
      await agentBook.lookupHuman(
        verification.address
      );

    if (!humanId) {
      console.log("AgentBook: UNREGISTERED");
      console.log("Prism PoUP: DENIED");

      json(res, 403, {
        authenticated: true,
        agent: verification.address,
        agentBook: "UNREGISTERED",
        humanBacked: false,
        poup: "DENIED",
      });

      return;
    }

    const humanReference = createHash("sha256")
      .update(`prism-agentkit:${humanId}`)
      .digest("hex");

    console.log("AgentBook: REGISTERED");
    console.log("Prism PoUP: ALLOWED");

    json(res, 200, {
      authenticated: true,
      agent: verification.address,
      agentBook: "REGISTERED",
      humanBacked: true,
      humanReference:
        `sha256:${humanReference}`,
      poup: "ALLOWED",
    });
  } catch (error) {
    console.error("AgentKit server error:", error);

    json(res, 400, {
      authenticated: false,
      error:
        error instanceof Error
          ? error.message
          : "unknown_error",
      poup: "DENIED",
    });
  }
});

server.listen(PORT, HOST, () => {
  console.log(
    "========================================"
  );
  console.log(
    "       PRISM AGENTKIT POUP SERVER"
  );
  console.log(
    "========================================"
  );
  console.log();
  console.log("Resource:", RESOURCE);
  console.log(
    "Waiting for AgentKit requests..."
  );
});
