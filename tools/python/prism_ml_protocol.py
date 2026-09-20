"""Prism v0.40 quantized Task, Job, wallet and signed Proof v2 wire formats.

Based on the Go sources supplied at commit b83445e. Signing API:
https://cryptography.io/en/latest/hazmat/primitives/asymmetric/ed25519/
"""
from __future__ import annotations

from dataclasses import dataclass, field
import hashlib
import json
from pathlib import Path
import re

from cryptography.exceptions import InvalidSignature
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey, Ed25519PublicKey

from prism_ml_runtime import InferenceError, infer, input_hash, normalize_payload

TYPE = "ml_inference_quantized"
MAX_ELEMENTS = 4096
MAX_WORK = 1 << 20
UINT64_MAX = (1 << 64) - 1
TASK_FIELDS = {"id", "type", "values", "values_b", "signed_values", "signed_values_b",
               "biases", "rows_a", "cols_a", "cols_b", "input_hash"}


class ProtocolError(ValueError):
    pass


def compact(value) -> bytes:
    return json.dumps(value, separators=(",", ":"), ensure_ascii=True, allow_nan=False).encode("ascii")


def digest(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def uint(value, label: str, minimum: int = 0) -> int:
    if type(value) is not int or not minimum <= value <= UINT64_MAX:
        raise ProtocolError(f"{label}: invalid uint64")
    return value


def hex_bytes(value, size: int, label: str) -> bytes:
    if not isinstance(value, str) or re.fullmatch(r"[0-9a-fA-F]{" + str(size * 2) + "}", value) is None:
        # Do not include the value: this also handles private key material.
        raise ProtocolError(f"{label}: invalid hex encoding or length")
    return bytes.fromhex(value)


def identifier(value, label: str = "ID") -> str:
    hex_bytes(value, 32, label)
    if value != value.lower():
        raise ProtocolError(f"{label}: expected lowercase hex")
    return value


def address(public: bytes) -> str:
    return "prism_" + hashlib.sha256(public).digest()[:20].hex()


def _unique(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ProtocolError("duplicate JSON key")
        result[key] = value
    return result


def _nonfinite(_value):
    raise ProtocolError("non-finite JSON number")


def decode_json(raw: bytes):
    try:
        return json.loads(raw.decode("utf-8-sig"), object_pairs_hook=_unique, parse_constant=_nonfinite)
    except (UnicodeError, json.JSONDecodeError, RecursionError) as error:
        raise ProtocolError("invalid JSON document") from error


def read_json(path: Path, limit: int = 1 << 20):
    with path.open("rb") as stream:
        raw = stream.read(limit + 1)
    if len(raw) > limit:
        raise ProtocolError("local JSON file exceeds size limit")
    return decode_json(raw)


def validate_payload(payload: object) -> dict:
    try:
        result = normalize_payload(payload)
    except InferenceError as error:
        raise ProtocolError(str(error)) from error
    batch, features, classes = (result[k] for k in ("batch_size", "features", "classes"))
    if max(batch * features, classes * features, batch, classes) > MAX_ELEMENTS:
        raise ProtocolError("quantized task exceeds Go element limit (4096)")
    if batch * features * classes > MAX_WORK:
        raise ProtocolError("quantized task exceeds Go work limit (1048576)")
    return result


def make_task(payload: object) -> dict:
    p = validate_payload(payload)
    hashed = input_hash(p)
    return {
        "id": digest(f"{TYPE}|{hashed}".encode("ascii")), "type": TYPE, "values": None,
        "signed_values": p["inputs"], "signed_values_b": p["weights"], "biases": p["biases"],
        "rows_a": p["batch_size"], "cols_a": p["features"], "cols_b": p["classes"],
        "input_hash": hashed,
    }


def task_payload(task: object) -> dict:
    if not isinstance(task, dict) or set(task) - TASK_FIELDS or task.get("type") != TYPE:
        raise ProtocolError("unsupported or malformed quantized task")
    for key in ("values", "values_b"):
        if task.get(key) is not None and task.get(key) != []:
            raise ProtocolError("quantized task must not contain unsigned values")
    payload = validate_payload({
        "batch_size": task.get("rows_a"), "features": task.get("cols_a"), "classes": task.get("cols_b"),
        "inputs": task.get("signed_values"), "weights": task.get("signed_values_b"), "biases": task.get("biases"),
    })
    expected = make_task(payload)
    if task.get("id") != expected["id"] or task.get("input_hash") != expected["input_hash"]:
        raise ProtocolError("task ID or input hash mismatch")
    return payload


def work_units(task: dict) -> int:
    p = task_payload(task)
    return p["batch_size"] * p["features"] * p["classes"]


@dataclass(frozen=True)
class Wallet:
    name: str
    address: str
    public_key: str
    private_key: Ed25519PrivateKey = field(repr=False, compare=False)


def wallet_from_record(record: object) -> Wallet:
    if not isinstance(record, dict) or not isinstance(record.get("name"), str) or not record["name"]:
        raise ProtocolError("invalid wallet record")
    public = hex_bytes(record.get("public_key"), 32, "wallet public key")
    private = hex_bytes(record.get("private_key"), 64, "wallet private key")
    # Go serializes seed || public key; cryptography imports the 32-byte seed.
    key = Ed25519PrivateKey.from_private_bytes(private[:32])
    derived = key.public_key().public_bytes_raw()
    if derived != public or private[32:] != public:
        raise ProtocolError("wallet seed, private-key suffix and public key do not match")
    expected_address = address(public)
    if record.get("address") != expected_address:
        raise ProtocolError("wallet address does not match public key")
    return Wallet(record["name"], expected_address, public.hex(), key)


def load_wallet(data: Path, name: str) -> Wallet:
    records = read_json(data / "wallets.json")
    if not isinstance(records, list):
        raise ProtocolError("wallets.json must contain a list")
    matching = [record for record in records if isinstance(record, dict) and record.get("name") == name]
    if len(matching) != 1:
        raise ProtocolError("wallet name absent or duplicated")
    return wallet_from_record(matching[0])


COMPUTE_PROOF_VERSION = 2


def compute_proof_id(proof: dict) -> str:
    message = (f'Prism/PoUW/Proof/v2|{proof["job_id"]}|{proof["chain_id"]}|'
               f'{proof["genesis_hash"]}|{proof["task"]["id"]}|{proof["worker"]}|'
               f'{proof["public_key"]}|{proof["result"]}|{proof["output_hash"]}|'
               f'{proof["score"]}')
    return digest(message.encode("utf-8"))


def proof_id(proof: dict) -> str:
    if proof.get("proof_version") == COMPUTE_PROOF_VERSION:
        return compute_proof_id(proof)
    message = (f'{proof["task"]["id"]}|{proof["worker"]}|{proof["public_key"]}|'
               f'{proof["result"]}|{proof["output_hash"]}|{proof["score"]}')
    return digest(message.encode("utf-8"))


def make_proof(task: dict, wallet: Wallet, *, job_id: str | None = None,
               chain_id: str | None = None,
               genesis_hash: str | None = None) -> dict:
    payload = task_payload(task)
    try:
        result = infer(payload)
    except InferenceError as error:
        raise ProtocolError(str(error)) from error
    predictions = result["predictions"]
    proof = {
        "task": make_task(payload), "worker": wallet.address, "public_key": wallet.public_key,
        "result": 0, "result_values": predictions, "output_hash": digest(compact(predictions)),
        "score": work_units(task),
    }
    context = (job_id, chain_id, genesis_hash)
    if any(value is not None for value in context):
        if any(value is None for value in context):
            raise ProtocolError("compute proof context is incomplete")
        identifier(job_id, "job ID")
        if (not isinstance(chain_id, str) or not chain_id.strip() or
                len(chain_id) > 128):
            raise ProtocolError("invalid chain ID")
        identifier(genesis_hash, "genesis hash")
        proof.update({
            "proof_version": COMPUTE_PROOF_VERSION,
            "job_id": job_id,
            "chain_id": chain_id,
            "genesis_hash": genesis_hash,
        })
    proof["id"] = proof_id(proof)
    proof["signature"] = wallet.private_key.sign(proof["id"].encode("ascii")).hex()
    return proof


def verify_proof(proof: dict) -> None:
    if not isinstance(proof, dict):
        raise ProtocolError("invalid proof object")
    try:
        version = proof.get("proof_version", 0)
        if type(version) is not int or version not in (0, COMPUTE_PROOF_VERSION):
            raise ProtocolError("unsupported proof version")
        if version == COMPUTE_PROOF_VERSION:
            identifier(proof.get("job_id"), "job ID")
            if (not isinstance(proof.get("chain_id"), str) or
                    not proof["chain_id"].strip() or
                    len(proof["chain_id"]) > 128):
                raise ProtocolError("invalid chain ID")
            identifier(proof.get("genesis_hash"), "genesis hash")
        payload = task_payload(proof["task"])
        predictions = infer(payload)["predictions"]
        values = proof["result_values"]
        if not isinstance(values, list):
            raise ProtocolError("invalid prediction array")
        for value in values:
            uint(value, "prediction")
        if values != predictions or uint(proof["result"], "scalar result") != 0:
            raise ProtocolError("proof result mismatch")
        if proof["output_hash"] != digest(compact(predictions)):
            raise ProtocolError("proof output hash mismatch")
        if uint(proof["score"], "score") != work_units(proof["task"]):
            raise ProtocolError("proof score mismatch")
        public = hex_bytes(proof["public_key"], 32, "proof public key")
        if proof["worker"] != address(public):
            raise ProtocolError("proof worker mismatch")
        if identifier(proof["id"], "proof ID") != proof_id(proof):
            raise ProtocolError("proof ID mismatch")
        signature = hex_bytes(proof["signature"], 64, "signature")
        Ed25519PublicKey.from_public_bytes(public).verify(signature, proof["id"].encode("ascii"))
    except (KeyError, InvalidSignature, InferenceError) as error:
        raise ProtocolError("invalid or incomplete signed proof") from error


def job_id(task_id: str, requester: str, reward: int, nonce: int) -> str:
    return digest(f"{task_id}|{requester}|{reward}|{nonce}".encode("utf-8"))


def validate_job(job: object, expected_id: str | None = None) -> dict:
    if not isinstance(job, dict):
        raise ProtocolError("missing job object")
    task = job.get("task")
    task_payload(task)
    requester = job.get("requester")
    if not isinstance(requester, str) or not requester.strip() or requester != requester.strip():
        raise ProtocolError("invalid job requester")
    reward = uint(job.get("reward"), "reward", 1)
    nonce = uint(job.get("nonce"), "nonce")
    calculated = job_id(task["id"], requester, reward, nonce)
    if job.get("id") != calculated or (expected_id is not None and calculated != expected_id):
        raise ProtocolError("job ID mismatch")
    if uint(job.get("workUnits"), "work units") != work_units(task):
        raise ProtocolError("job work units mismatch")
    status, worker, proof = job.get("status"), job.get("worker", ""), job.get("proofId", "")
    if status == "OPEN":
        if worker or proof:
            raise ProtocolError("OPEN job already contains worker or proof")
    elif status in ("CLAIMED", "VERIFIED"):
        if not isinstance(worker, str) or re.fullmatch(r"prism_[0-9a-f]{40}", worker) is None:
            raise ProtocolError("invalid job worker")
        if status == "VERIFIED":
            identifier(proof, "job proof ID")
        elif proof:
            raise ProtocolError("CLAIMED job already contains proof")
    else:
        raise ProtocolError("unknown job status")
    return job
