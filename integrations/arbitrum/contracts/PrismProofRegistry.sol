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

    // Prism Proof Registry V2 metadata.
    mapping(bytes32 => bytes32) private prismChainIdByJobId;
    mapping(bytes32 => bytes32) private registryIdByJobId;
    mapping(bytes32 => bytes32) private jobByRegistryId;

    uint256 public proofCount;

    error Unauthorized();
    error InvalidJobId();
    error InvalidProofId();
    error InvalidWorkerIdHash();
    error InvalidPrismChainIdHash();

    error JobAlreadyRegistered(bytes32 jobId);
    error ProofAlreadyRegistered(bytes32 proofId);
    error RegistryAlreadyRegistered(bytes32 registryId);

    event RecorderUpdated(
        address indexed recorder,
        bool allowed
    );

    /// @notice Original Prism proof event.
    /// @dev Kept unchanged for backwards compatibility.
    event ProofRegistered(
        bytes32 indexed jobId,
        bytes32 indexed proofId,
        bytes32 indexed workerIdHash,
        address submitter,
        uint64 registeredAt
    );

    /// @notice Emitted when a Proof v2 anchor is bound to a Prism chain.
    event ProofAnchorRegistered(
        bytes32 indexed registryId,
        bytes32 indexed jobId,
        bytes32 indexed proofId,
        bytes32 prismChainIdHash
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

    /// @notice Legacy proof registration entrypoint.
    /// @dev Preserved so existing Prism tooling remains compatible.
    function registerProof(
        bytes32 jobId,
        bytes32 proofId,
        bytes32 workerIdHash
    ) external onlyRecorder {
        _registerProof(
            jobId,
            proofId,
            workerIdHash,
            bytes32(0)
        );
    }

    /// @notice Register a Prism Proof v2 anchor bound to a Prism chain.
    /// @return registryId Deterministic identifier for the cross-chain anchor.
    function registerProofV2(
        bytes32 jobId,
        bytes32 proofId,
        bytes32 workerIdHash,
        bytes32 prismChainIdHash
    ) external onlyRecorder returns (bytes32 registryId) {
        if (prismChainIdHash == bytes32(0)) {
            revert InvalidPrismChainIdHash();
        }

        registryId = computeRegistryId(
            jobId,
            prismChainIdHash,
            proofId
        );

        _registerProof(
            jobId,
            proofId,
            workerIdHash,
            prismChainIdHash
        );
    }

    function _registerProof(
        bytes32 jobId,
        bytes32 proofId,
        bytes32 workerIdHash,
        bytes32 prismChainIdHash
    ) internal {
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

        bytes32 registryId;

        if (prismChainIdHash != bytes32(0)) {
            registryId = computeRegistryId(
                jobId,
                prismChainIdHash,
                proofId
            );

            if (jobByRegistryId[registryId] != bytes32(0)) {
                revert RegistryAlreadyRegistered(registryId);
            }
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

        if (prismChainIdHash != bytes32(0)) {
            prismChainIdByJobId[jobId] = prismChainIdHash;
            registryIdByJobId[jobId] = registryId;
            jobByRegistryId[registryId] = jobId;

            emit ProofAnchorRegistered(
                registryId,
                jobId,
                proofId,
                prismChainIdHash
            );
        }

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

    function getPrismChainIdHash(
        bytes32 jobId
    ) external view returns (bytes32) {
        return prismChainIdByJobId[jobId];
    }

    function getRegistryId(
        bytes32 jobId
    ) external view returns (bytes32) {
        return registryIdByJobId[jobId];
    }

    function getJobForRegistry(
        bytes32 registryId
    ) external view returns (bytes32) {
        return jobByRegistryId[registryId];
    }

    function computeRegistryId(
        bytes32 jobId,
        bytes32 prismChainIdHash,
        bytes32 proofId
    ) public pure returns (bytes32) {
        return keccak256(
            abi.encode(
                jobId,
                prismChainIdHash,
                proofId
            )
        );
    }
}
