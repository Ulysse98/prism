use anchor_lang::prelude::*;
use solana_keccak_hasher::hashv;

declare_id!("2yjpnNnDRnK4pvyRWLLMftAH2jfuAC2TnjyArVW3bhPz");

const REGISTRY_SEED: &[u8] = b"registry";
const JOB_SEED: &[u8] = b"job";
const PROOF_SEED: &[u8] = b"proof";

#[program]
pub mod prism_proof_registry {
    use super::*;

    pub fn initialize_registry(ctx: Context<InitializeRegistry>) -> Result<()> {
        let config = &mut ctx.accounts.config;

        config.owner = ctx.accounts.owner.key();
        config.recorder = ctx.accounts.owner.key();
        config.proof_count = 0;
        config.bump = ctx.bumps.config;

        emit!(RecorderUpdated {
            recorder: config.recorder,
        });

        Ok(())
    }

    pub fn set_recorder(ctx: Context<SetRecorder>, new_recorder: Pubkey) -> Result<()> {
        require!(
            new_recorder != Pubkey::default(),
            PrismRegistryError::InvalidRecorder
        );

        ctx.accounts.config.recorder = new_recorder;

        emit!(RecorderUpdated {
            recorder: new_recorder,
        });

        Ok(())
    }

    pub fn register_proof_v2(
        ctx: Context<RegisterProofV2>,
        job_id: [u8; 32],
        proof_id: [u8; 32],
        worker_id_hash: [u8; 32],
        prism_chain_id_hash: [u8; 32],
    ) -> Result<()> {
        require!(job_id != [0u8; 32], PrismRegistryError::InvalidJobId);

        require!(proof_id != [0u8; 32], PrismRegistryError::InvalidProofId);

        require!(
            worker_id_hash != [0u8; 32],
            PrismRegistryError::InvalidWorkerIdHash
        );

        require!(
            prism_chain_id_hash != [0u8; 32],
            PrismRegistryError::InvalidPrismChainIdHash
        );

        // Matches the cross-chain Prism Registry V2 identity:
        //
        // keccak256(
        //     job_id ||
        //     prism_chain_id_hash ||
        //     proof_id
        // )
        //
        // All three values are fixed 32-byte values.
        let registry_id = hashv(&[
            job_id.as_ref(),
            prism_chain_id_hash.as_ref(),
            proof_id.as_ref(),
        ])
        .to_bytes();

        let registered_at = Clock::get()?.unix_timestamp;

        let proof = &mut ctx.accounts.proof;

        proof.job_id = job_id;
        proof.proof_id = proof_id;
        proof.worker_id_hash = worker_id_hash;
        proof.prism_chain_id_hash = prism_chain_id_hash;
        proof.registry_id = registry_id;
        proof.submitter = ctx.accounts.recorder.key();
        proof.registered_at = registered_at;
        proof.bump = ctx.bumps.proof;

        // Separate PDA claiming this proof ID.
        // This prevents the same Proof v2 ID from being anchored
        // under a different Prism compute Job ID.
        let proof_index = &mut ctx.accounts.proof_index;

        proof_index.job_id = job_id;
        proof_index.bump = ctx.bumps.proof_index;

        let config = &mut ctx.accounts.config;

        config.proof_count = config
            .proof_count
            .checked_add(1)
            .ok_or(PrismRegistryError::ProofCountOverflow)?;

        emit!(ProofRegistered {
            registry_id,
            job_id,
            proof_id,
            worker_id_hash,
            prism_chain_id_hash,
            submitter: ctx.accounts.recorder.key(),
            registered_at,
        });

        Ok(())
    }
}

#[derive(Accounts)]
pub struct InitializeRegistry<'info> {
    #[account(
        init,
        payer = owner,
        space = RegistryConfig::SPACE,
        seeds = [REGISTRY_SEED],
        bump
    )]
    pub config: Account<'info, RegistryConfig>,

    #[account(mut)]
    pub owner: Signer<'info>,

    pub system_program: Program<'info, System>,
}

#[derive(Accounts)]
pub struct SetRecorder<'info> {
    #[account(
        mut,
        seeds = [REGISTRY_SEED],
        bump = config.bump,
        has_one = owner @ PrismRegistryError::Unauthorized
    )]
    pub config: Account<'info, RegistryConfig>,

    pub owner: Signer<'info>,
}

#[derive(Accounts)]
#[instruction(
    job_id: [u8; 32],
    proof_id: [u8; 32]
)]
pub struct RegisterProofV2<'info> {
    #[account(
        mut,
        seeds = [REGISTRY_SEED],
        bump = config.bump,
        has_one = recorder @ PrismRegistryError::UnauthorizedRecorder
    )]
    pub config: Account<'info, RegistryConfig>,

    #[account(
        init,
        payer = recorder,
        space = ProofRecord::SPACE,
        seeds = [JOB_SEED, job_id.as_ref()],
        bump
    )]
    pub proof: Account<'info, ProofRecord>,

    #[account(
        init,
        payer = recorder,
        space = ProofIndex::SPACE,
        seeds = [PROOF_SEED, proof_id.as_ref()],
        bump
    )]
    pub proof_index: Account<'info, ProofIndex>,

    #[account(mut)]
    pub recorder: Signer<'info>,

    pub system_program: Program<'info, System>,
}

#[account]
pub struct RegistryConfig {
    pub owner: Pubkey,
    pub recorder: Pubkey,
    pub proof_count: u64,
    pub bump: u8,
}

impl RegistryConfig {
    pub const SPACE: usize = 8 +  // discriminator
        32 + // owner
        32 + // recorder
        8 +  // proof_count
        1; // bump
}

#[account]
pub struct ProofRecord {
    pub job_id: [u8; 32],
    pub proof_id: [u8; 32],
    pub worker_id_hash: [u8; 32],
    pub prism_chain_id_hash: [u8; 32],
    pub registry_id: [u8; 32],
    pub submitter: Pubkey,
    pub registered_at: i64,
    pub bump: u8,
}

impl ProofRecord {
    pub const SPACE: usize = 8 +       // discriminator
        (32 * 5) + // hashes / IDs
        32 +      // submitter
        8 +       // registered_at
        1; // bump
}

#[account]
pub struct ProofIndex {
    pub job_id: [u8; 32],
    pub bump: u8,
}

impl ProofIndex {
    pub const SPACE: usize = 8 +  // discriminator
        32 + // job_id
        1; // bump
}

#[event]
pub struct RecorderUpdated {
    pub recorder: Pubkey,
}

#[event]
pub struct ProofRegistered {
    pub registry_id: [u8; 32],
    pub job_id: [u8; 32],
    pub proof_id: [u8; 32],
    pub worker_id_hash: [u8; 32],
    pub prism_chain_id_hash: [u8; 32],
    pub submitter: Pubkey,
    pub registered_at: i64,
}

#[error_code]
pub enum PrismRegistryError {
    #[msg("Unauthorized registry owner")]
    Unauthorized,

    #[msg("Unauthorized Prism proof recorder")]
    UnauthorizedRecorder,

    #[msg("Recorder public key cannot be zero")]
    InvalidRecorder,

    #[msg("Prism Job ID cannot be zero")]
    InvalidJobId,

    #[msg("Prism Proof ID cannot be zero")]
    InvalidProofId,

    #[msg("Prism worker identity hash cannot be zero")]
    InvalidWorkerIdHash,

    #[msg("Prism chain ID hash cannot be zero")]
    InvalidPrismChainIdHash,

    #[msg("Proof counter overflow")]
    ProofCountOverflow,
}
