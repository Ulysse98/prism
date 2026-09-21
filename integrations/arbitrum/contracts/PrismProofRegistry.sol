// SPDX-License-Identifier: MIT
pragma solidity ^0.8.34;

/// @title PrismProofRegistry
/// @notice Anchors verified Prism Proof v2 compute proofs on Arbitrum.
contract PrismProofRegistry {
    struct ProofRecord {
        bytes32 proofId;
        bytes32 workerIdHash;
        address submitter;
        uint64 registeredAt;
        bool exists;
    }

    address public owner;

    mapping(address => bool) public recorders;
    mapping(bytes32 => ProofRecord) private proofsByJobId;
    mapping(bytes32 => bytes32) private jobByProofId;

    uint256 public proofCount;

    error Unauthorized();
    error InvalidJobId();
    error InvalidProofId();
    error InvalidWorkerIdHash();
    error JobAlreadyRegistered(bytes32 jobId);
    error ProofAlreadyRegistered(bytes32 proofId);

    event RecorderUpdated(
        address indexed recorder,
        bool allowed
    );

    event ProofRegistered(
        bytes32 indexed jobId,
        bytes32 indexed proofId,
        bytes32 indexed workerIdHash,
        address submitter,
        uint64 registeredAt
    );

    constructor() {
        owner = msg.sender;
        recorders[msg.sender] = true;

        emit RecorderUpdated(msg.sender, true);
    }

    modifier onlyOwner() {
        if (msg.sender != owner) {
            revert Unauthorized();
        }

        _;
    }

    modifier onlyRecorder() {
        if (!recorders[msg.sender]) {
            revert Unauthorized();
        }

        _;
    }

    function setRecorder(
        address recorder,
        bool allowed
    ) external onlyOwner {
        recorders[recorder] = allowed;

        emit RecorderUpdated(recorder, allowed);
    }

    function registerProof(
        bytes32 jobId,
        bytes32 proofId,
        bytes32 workerIdHash
    ) external onlyRecorder {
        if (jobId == bytes32(0)) {
            revert InvalidJobId();
        }

        if (proofId == bytes32(0)) {
            revert InvalidProofId();
        }

        if (workerIdHash == bytes32(0)) {
            revert InvalidWorkerIdHash();
        }

        if (proofsByJobId[jobId].exists) {
            revert JobAlreadyRegistered(jobId);
        }

        if (jobByProofId[proofId] != bytes32(0)) {
            revert ProofAlreadyRegistered(proofId);
        }

        uint64 registeredAt = uint64(block.timestamp);

        proofsByJobId[jobId] = ProofRecord({
            proofId: proofId,
            workerIdHash: workerIdHash,
            submitter: msg.sender,
            registeredAt: registeredAt,
            exists: true
        });

        jobByProofId[proofId] = jobId;
        proofCount++;

        emit ProofRegistered(
            jobId,
            proofId,
            workerIdHash,
            msg.sender,
            registeredAt
        );
    }

    function getProof(
        bytes32 jobId
    ) external view returns (ProofRecord memory) {
        return proofsByJobId[jobId];
    }

    function getJobForProof(
        bytes32 proofId
    ) external view returns (bytes32) {
        return jobByProofId[proofId];
    }

    function hasProof(
        bytes32 jobId
    ) external view returns (bool) {
        return proofsByJobId[jobId].exists;
    }
}
