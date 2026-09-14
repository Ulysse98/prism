// SPDX-License-Identifier: MIT
pragma solidity ^0.8.34;

/// @title PrismWorkRegistry
/// @notice Anchors Prism Proof-of-Useful-Work results onchain.
contract PrismWorkRegistry {
    struct UsefulWorkProof {
        address worker;
        bytes32 proofHash;
        string workType;
        uint256 score;
        uint64 timestamp;
    }

    address public owner;

    mapping(address => bool) public recorders;
    mapping(bytes32 => UsefulWorkProof) private proofs;

    uint256 public proofCount;

    error Unauthorized();
    error InvalidWorker();
    error InvalidProofHash();
    error InvalidWorkType();
    error ProofAlreadyExists(bytes32 proofHash);

    event RecorderUpdated(
        address indexed recorder,
        bool allowed
    );

    event UsefulWorkRecorded(
        bytes32 indexed proofHash,
        address indexed worker,
        string workType,
        uint256 score,
        uint64 timestamp
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

    function recordUsefulWork(
        address worker,
        bytes32 proofHash,
        string calldata workType,
        uint256 score
    ) external onlyRecorder {
        if (worker == address(0)) {
            revert InvalidWorker();
        }

        if (proofHash == bytes32(0)) {
            revert InvalidProofHash();
        }

        if (bytes(workType).length == 0) {
            revert InvalidWorkType();
        }

        if (proofs[proofHash].timestamp != 0) {
            revert ProofAlreadyExists(proofHash);
        }

        uint64 recordedAt = uint64(block.timestamp);

        proofs[proofHash] = UsefulWorkProof({
            worker: worker,
            proofHash: proofHash,
            workType: workType,
            score: score,
            timestamp: recordedAt
        });

        proofCount++;

        emit UsefulWorkRecorded(
            proofHash,
            worker,
            workType,
            score,
            recordedAt
        );
    }

    function getProof(
        bytes32 proofHash
    ) external view returns (UsefulWorkProof memory) {
        return proofs[proofHash];
    }

    function proofExists(
        bytes32 proofHash
    ) external view returns (bool) {
        return proofs[proofHash].timestamp != 0;
    }
}