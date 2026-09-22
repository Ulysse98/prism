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
fn test_registry_happy_path() {
    let program_id = prism_proof_registry::id();
    let owner = Keypair::new();

    let (config, _) = Pubkey::find_program_address(&[b"registry"], &program_id);

    let mut svm = LiteSVM::new();

    let program_bytes = include_bytes!(concat!(
        env!("CARGO_TARGET_TMPDIR"),
        "/../deploy/prism_proof_registry.so"
    ));

    svm.add_program(program_id, program_bytes).unwrap();

    svm.airdrop(&owner.pubkey(), 10_000_000_000).unwrap();

    // --------------------------------------------------------
    // Initialize registry
    // --------------------------------------------------------

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

    let config_account = svm
        .get_account(&config)
        .expect("registry config should exist");

    let mut config_data: &[u8] = &config_account.data;

    let config_state =
        prism_proof_registry::RegistryConfig::try_deserialize(&mut config_data).unwrap();

    assert_eq!(config_state.owner, owner.pubkey(),);

    assert_eq!(config_state.recorder, owner.pubkey(),);

    assert_eq!(config_state.proof_count, 0,);

    // --------------------------------------------------------
    // Prism Proof v2
    // --------------------------------------------------------

    let job_id = [1u8; 32];
    let proof_id = [2u8; 32];
    let worker_id_hash = [3u8; 32];
    let prism_chain_id_hash = [4u8; 32];

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

    // --------------------------------------------------------
    // Read ProofRecord PDA
    // --------------------------------------------------------

    let proof_account = svm.get_account(&proof).expect("proof PDA should exist");

    let mut proof_data: &[u8] = &proof_account.data;

    let proof_state = prism_proof_registry::ProofRecord::try_deserialize(&mut proof_data).unwrap();

    assert_eq!(proof_state.job_id, job_id);
    assert_eq!(proof_state.proof_id, proof_id);

    assert_eq!(proof_state.worker_id_hash, worker_id_hash,);

    assert_eq!(proof_state.prism_chain_id_hash, prism_chain_id_hash,);

    assert_eq!(proof_state.submitter, owner.pubkey(),);

    // --------------------------------------------------------
    // Cross-chain registry ID
    // --------------------------------------------------------

    let expected_registry_id = solana_keccak_hasher::hashv(&[
        job_id.as_ref(),
        prism_chain_id_hash.as_ref(),
        proof_id.as_ref(),
    ])
    .to_bytes();

    assert_eq!(proof_state.registry_id, expected_registry_id,);

    // --------------------------------------------------------
    // Read ProofIndex PDA
    // --------------------------------------------------------

    let proof_index_account = svm
        .get_account(&proof_index)
        .expect("proof index PDA should exist");

    let mut index_data: &[u8] = &proof_index_account.data;

    let index_state = prism_proof_registry::ProofIndex::try_deserialize(&mut index_data).unwrap();

    assert_eq!(index_state.job_id, job_id,);

    // --------------------------------------------------------
    // proof_count == 1
    // --------------------------------------------------------

    let config_account = svm.get_account(&config).unwrap();

    let mut config_data: &[u8] = &config_account.data;

    let config_state =
        prism_proof_registry::RegistryConfig::try_deserialize(&mut config_data).unwrap();

    assert_eq!(config_state.proof_count, 1,);
}
