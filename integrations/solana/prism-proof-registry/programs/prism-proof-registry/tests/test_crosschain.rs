use {
    anchor_lang::{
        prelude::Pubkey,
        solana_program::{instruction::Instruction, system_program},
        AccountDeserialize, InstructionData, ToAccountMetas,
    },
    litesvm::LiteSVM,
    solana_keypair::Keypair,
    solana_message::{Message, VersionedMessage},
    solana_signer::Signer,
    solana_transaction::versioned::VersionedTransaction,
};

fn send_ix(svm: &mut LiteSVM, payer: &Keypair, instruction: Instruction) {
    let blockhash = svm.latest_blockhash();

    let message = Message::new_with_blockhash(&[instruction], Some(&payer.pubkey()), &blockhash);

    let tx = VersionedTransaction::try_new(VersionedMessage::Legacy(message), &[payer]).unwrap();

    svm.send_transaction(tx).unwrap();
}

#[test]
fn matches_arbitrum_registry_v2_vector() {
    let program_id = prism_proof_registry::id();
    let owner = Keypair::new();

    let mut svm = LiteSVM::new();

    let program_bytes = include_bytes!(concat!(
        env!("CARGO_TARGET_TMPDIR"),
        "/../deploy/prism_proof_registry.so"
    ));

    svm.add_program(program_id, program_bytes).unwrap();

    svm.airdrop(&owner.pubkey(), 10_000_000_000).unwrap();

    let (config, _) = Pubkey::find_program_address(&[b"registry"], &program_id);

    let initialize_ix = Instruction::new_with_bytes(
        program_id,
        &prism_proof_registry::instruction::InitializeRegistry {}.data(),
        prism_proof_registry::accounts::InitializeRegistry {
            config,
            owner: owner.pubkey(),
            system_program: system_program::ID,
        }
        .to_account_metas(None),
    );

    send_ix(&mut svm, &owner, initialize_ix);

    // Same canonical V2 vector used by the Arbitrum registry tests.
    let job_id = solana_keccak_hasher::hashv(&[b"prism-job-001"]).to_bytes();

    let proof_id = solana_keccak_hasher::hashv(&[b"prism-proof-v2-001"]).to_bytes();

    let worker_id_hash = solana_keccak_hasher::hashv(&[b"prism-worker-alice"]).to_bytes();

    let prism_chain_id_hash = solana_keccak_hasher::hashv(&[b"prism-d8c1f3e740b48957"]).to_bytes();

    // Known Keccak-256 values for the Arbitrum V2 vector.
    assert_eq!(
        job_id,
        [
            0xdb, 0xab, 0x0c, 0x89, 0xf7, 0x52, 0x68, 0x97, 0x48, 0xf4, 0xb1, 0xd3, 0x75, 0xcd,
            0x94, 0x80, 0x0b, 0x40, 0xa1, 0xed, 0x29, 0x9e, 0x02, 0x8e, 0xdf, 0xa2, 0x33, 0x6c,
            0x0b, 0xa9, 0xa6, 0xcd,
        ]
    );

    assert_eq!(
        proof_id,
        [
            0x42, 0xb5, 0xdf, 0x2e, 0x46, 0x7b, 0x31, 0x6c, 0xd4, 0xfe, 0x45, 0xbd, 0xc3, 0x03,
            0xc1, 0x54, 0x94, 0x85, 0x4b, 0x91, 0x7d, 0x8b, 0x98, 0xee, 0x82, 0x40, 0x2d, 0x1e,
            0xf9, 0x07, 0x6c, 0x38,
        ]
    );

    assert_eq!(
        worker_id_hash,
        [
            0x9e, 0xf8, 0xa0, 0xbd, 0x33, 0x8c, 0xcc, 0xf1, 0x54, 0x4c, 0x7c, 0x24, 0xb0, 0x1e,
            0x51, 0x49, 0xc9, 0x39, 0xe2, 0x54, 0xfa, 0x9c, 0x22, 0xbf, 0xc0, 0xc9, 0xa6, 0xfd,
            0xe7, 0x49, 0x67, 0x26,
        ]
    );

    assert_eq!(
        prism_chain_id_hash,
        [
            0x27, 0xa9, 0x55, 0xf0, 0xf0, 0x28, 0xe1, 0xb3, 0xf3, 0x97, 0xd2, 0x0c, 0x36, 0xa7,
            0x7c, 0x7b, 0x85, 0x46, 0xfe, 0x31, 0xdd, 0x75, 0x4b, 0xcf, 0x1f, 0xbd, 0xaa, 0xe3,
            0xad, 0x95, 0xae, 0xdb,
        ]
    );

    // Solidity:
    //
    // keccak256(
    //     abi.encode(
    //         jobId,
    //         prismChainIdHash,
    //         proofId
    //     )
    // )
    //
    // All arguments are bytes32, so ABI encoding is the
    // 96-byte concatenation of those three values.
    let expected_registry_id = [
        0x0d, 0x2f, 0x74, 0x11, 0xa6, 0xf9, 0xe0, 0x26, 0x32, 0x09, 0xbc, 0xc7, 0x27, 0x8d, 0x17,
        0x23, 0x46, 0xe0, 0xb5, 0x91, 0x69, 0x02, 0xbb, 0x22, 0xef, 0xc8, 0xc0, 0xff, 0xbb, 0x1a,
        0x61, 0x8e,
    ];

    let computed_registry_id = solana_keccak_hasher::hashv(&[
        job_id.as_ref(),
        prism_chain_id_hash.as_ref(),
        proof_id.as_ref(),
    ])
    .to_bytes();

    assert_eq!(computed_registry_id, expected_registry_id,);

    let (proof, _) = Pubkey::find_program_address(&[b"job", job_id.as_ref()], &program_id);

    let (proof_index, _) =
        Pubkey::find_program_address(&[b"proof", proof_id.as_ref()], &program_id);

    let register_ix = Instruction::new_with_bytes(
        program_id,
        &prism_proof_registry::instruction::RegisterProofV2 {
            job_id,
            proof_id,
            worker_id_hash,
            prism_chain_id_hash,
        }
        .data(),
        prism_proof_registry::accounts::RegisterProofV2 {
            config,
            proof,
            proof_index,
            recorder: owner.pubkey(),
            system_program: system_program::ID,
        }
        .to_account_metas(None),
    );

    send_ix(&mut svm, &owner, register_ix);

    let proof_account = svm.get_account(&proof).expect("proof record must exist");

    let mut proof_data: &[u8] = &proof_account.data;

    let proof_state = prism_proof_registry::ProofRecord::try_deserialize(&mut proof_data).unwrap();

    assert_eq!(proof_state.registry_id, expected_registry_id,);

    assert_eq!(proof_state.job_id, job_id,);

    assert_eq!(proof_state.proof_id, proof_id,);

    assert_eq!(proof_state.prism_chain_id_hash, prism_chain_id_hash,);
}
