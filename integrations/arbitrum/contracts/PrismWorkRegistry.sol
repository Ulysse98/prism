// SPDX-License-Identifier: MIT
pragma solidity ^0.8.34;

/// @title PrismWorkRegistry
/// @notice Anchors verified Prism Proof-of-Useful-Work proofs on an EVM chain.
contract PrismWorkRegistry {
    struct UsefulWorkProof {
        bytes32 proofId;
        string prismWorker;
        string workType;
        uint256 score;
        uint64 anchoredAt;
    }

    address public owner;

    mapping(address => bool) public recorders;
    mapping(bytes32 => UsefulWorkProof) private proofs;

    uint256 public proofCount;

    error Unauthorized();
    error InvalidWorker();
    error InvalidProofId();
    error InvalidWorkType();
    error ProofAlreadyExists(bytes32 proofId);

    event RecorderUpdated(
        address indexed recorder,
        bool allowed
    );

    event UsefulWorkRecorded(
        bytes32 indexed proofId,
        string prismWorker,
        string workType,
        uint256 score,
        uint64 anchoredAt
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
        string calldata prismWorker,
        bytes32 proofId,
        string calldata workType,
        uint256 score
    ) external onlyRecorder {
        if (bytes(prismWorker).length == 0) {
            revert InvalidWorker();
        }

        if (proofId == bytes32(0)) {
            revert InvalidProofId();
        }

        if (bytes(workType).length == 0) {
            revert InvalidWorkType();
        }

        if (proofs[proofId].anchoredAt != 0) {
            revert ProofAlreadyExists(proofId);
        }

        uint64 anchoredAt = uint64(block.timestamp);

        proofs[proofId] = UsefulWorkProof({
            proofId: proofId,
            prismWorker: prismWorker,
            workType: workType,
            score: score,
            anchoredAt: anchoredAt
        });

        proofCount++;

        emit UsefulWorkRecorded(
            proofId,
            prismWorker,
            workType,
            score,
            anchoredAt
        );
    }

    function getProof(
        bytes32 proofId
    ) external view returns (UsefulWorkProof memory) {
        return proofs[proofId];
    }

    function proofExists(
        bytes32 proofId
    ) external view returns (bool) {
        return proofs[proofId].anchoredAt != 0;
    }
}