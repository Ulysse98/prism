// ============================================================================
// Prism Explorer - JavaScript Application
// ============================================================================

// Configuration
const API_BASE_URL = window.location.origin.includes('localhost') 
    ? 'http://localhost:8080' 
    : '/api';

// State
let currentSection = 'dashboard';
let networkStatus = 'disconnected';
let statsInterval;
let blocksInterval;
let jobsInterval;
let validatorsInterval;

// DOM Elements
const sections = document.querySelectorAll('.section');
const navLinks = document.querySelectorAll('.nav-link');
const networkStatusDot = document.getElementById('network-status');

// ============================================================================
// Navigation
// ============================================================================
function initNavigation() {
    navLinks.forEach(link => {
        link.addEventListener('click', (e) => {
            e.preventDefault();
            const sectionId = link.dataset.section;
            switchSection(sectionId);
        });
    });
}

function switchSection(sectionId) {
    // Update nav links
    navLinks.forEach(link => {
        link.classList.remove('active');
        if (link.dataset.section === sectionId) {
            link.classList.add('active');
        }
    });

    // Update sections
    sections.forEach(section => {
        section.classList.remove('active');
        if (section.id === sectionId) {
            section.classList.add('active');
        }
    });

    currentSection = sectionId;

    // Load data for the section
    switch (sectionId) {
        case 'dashboard':
            loadDashboard();
            break;
        case 'blocks':
            loadBlocks();
            break;
        case 'jobs':
            loadJobs();
            break;
        case 'validators':
            loadValidators();
            break;
        case 'compute':
            // Compute section doesn't need data loading
            break;
    }
}

// ============================================================================
// Network Status
// ============================================================================
async function checkNetworkStatus() {
    try {
        const response = await fetch(`${API_BASE_URL}/api/v1/health`);
        if (response.ok) {
            const data = await response.json();
            networkStatus = 'connected';
            networkStatusDot.classList.add('connected');
            networkStatusDot.classList.remove('disconnected');
            return true;
        }
    } catch (error) {
        console.error('Network status check failed:', error);
    }
    
    networkStatus = 'disconnected';
    networkStatusDot.classList.add('disconnected');
    networkStatusDot.classList.remove('connected');
    return false;
}

// ============================================================================
// Dashboard
// ============================================================================
async function loadDashboard() {
    loadRecentActivity();
    if (!await checkNetworkStatus()) {
        showToast('Impossible de se connecter à l\'API Prism', 'error');
        return;
    }

    try {
        // Load status
        const statusResponse = await fetch(`${API_BASE_URL}/api/v1/status`);
        const statusData = await statusResponse.json();

        // Update stats
        document.getElementById('block-height').textContent = statusData.height || 0;
        document.getElementById('validator-count').textContent = statusData.validators || 0;
        document.getElementById('total-stake').textContent = formatNumber(statusData.totalStake || 0);
        document.getElementById('protocol-version').textContent = statusData.protocol || '0.36';
        document.getElementById('last-hash').textContent = statusData.lastHash ? 
            `${statusData.lastHash.substring(0, 16)}...` : '-';

        // Load validators count
        const validatorsResponse = await fetch(`${API_BASE_URL}/api/v1/validators`);
        const validatorsData = await validatorsResponse.json();
        document.getElementById('validator-count').textContent = validatorsData.validators?.length ?? 0;

        // Load jobs count
        const jobsResponse = await fetch(`${API_BASE_URL}/api/v1/compute/jobs`);
        const jobsData = await jobsResponse.json();
        document.getElementById('job-count').textContent = jobsData.length || 0;

    } catch (error) {
        console.error('Failed to load dashboard:', error);
        showToast('Erreur lors du chargement du dashboard', 'error');
    }
}

// ============================================================================
// Blocks
// ============================================================================
async function loadBlocks(limit = 50) {
    if (!await checkNetworkStatus()) return;

    const blocksBody = document.getElementById('blocks-body');
    blocksBody.innerHTML = '<tr class="loading-row"><td colspan="7">Chargement des blocks...</td></tr>';

    try {
        const response = await fetch(`${API_BASE_URL}/api/v1/status`);
        const data = await response.json();

        // For now, we'll display the chain info
        // In a full implementation, we'd fetch the actual blocks
        const chain = data.blockchain || {};
        const blocks = chain.Blocks || [];

        if (blocks.length === 0) {
            blocksBody.innerHTML = '<tr><td colspan="7">Aucun block trouvé</td></tr>';
            return;
        }

        // Display last N blocks
        const displayBlocks = blocks.slice(-Math.min(limit, blocks.length));
        blocksBody.innerHTML = displayBlocks.map(block => `
            <tr>
                <td>${block.Height || '-'}</td>
                <td class="hash">${block.Hash ? `${block.Hash.substring(0, 16)}...` : '-'}</td>
                <td>${block.Proposer || '-'}</td>
                <td>${block.Transactions ? block.Transactions.length : 0}</td>
                <td>${block.UsefulWork ? block.UsefulWork.length : 0}</td>
                <td>${new Date(block.Timestamp * 1000).toLocaleString()}</td>
                <td><button class="btn btn-secondary" onclick="viewBlock('${block.Hash}')">Voir</button></td>
            </tr>
        `).join('');

    } catch (error) {
        console.error('Failed to load blocks:', error);
        blocksBody.innerHTML = '<tr><td colspan="7">Erreur lors du chargement</td></tr>';
    }
}

function searchBlocks() {
    const query = document.getElementById('block-search').value.toLowerCase();
    // Implement block search
    showToast('Recherche de blocks: ' + query, 'info');
}

function viewBlock(hash) {
    showToast('Affichage du block: ' + hash.substring(0, 16) + '...', 'info');
    // In a full implementation, this would navigate to a block detail page
}

// ============================================================================
// Jobs (ML Tasks)
// ============================================================================
async function loadJobs() {
    if (!await checkNetworkStatus()) return;

    const jobsBody = document.getElementById('jobs-body');
    jobsBody.innerHTML = '<tr class="loading-row"><td colspan="7">Chargement des jobs...</td></tr>';

    try {
        const response = await fetch(`${API_BASE_URL}/api/v1/compute/jobs`);
        const jobs = await response.json();

        if (!jobs || jobs.length === 0) {
            jobsBody.innerHTML = '<tr><td colspan="7">Aucun job trouvé</td></tr>';
            return;
        }

        // Sort by creation date (newest first)
        jobs.sort((a, b) => (b.createdAt || 0) - (a.createdAt || 0));

        jobsBody.innerHTML = jobs.map(job => `
            <tr>
                <td class="hash">${job.id ? job.id.substring(0, 16) + '...' : '-'}</td>
                <td>${job.task?.type || job.type || '-'}</td>
                <td>${job.requester || '-'}</td>
                <td>${job.worker || '-'}</td>
                <td>${formatNumber(job.reward || 0)}</td>
                <td><span class="status-badge ${job.status || 'open'}">${job.status || 'open'}</span></td>
                <td>
                    ${job.status === 'open' ? 
                        `<button class="btn btn-primary" onclick="claimJob('${job.id}')">Réclamer</button>` : ''}
                    <button class="btn btn-secondary" onclick="viewJob('${job.id}')">Voir</button>
                </td>
            </tr>
        `).join('');

    } catch (error) {
        console.error('Failed to load jobs:', error);
        jobsBody.innerHTML = '<tr><td colspan="7">Erreur lors du chargement</td></tr>';
    }
}

function filterJobs() {
    const status = document.getElementById('job-status-filter').value;
    loadJobs(); // In a full implementation, we'd filter the jobs
    showToast('Filtre appliqué: ' + status, 'info');
}

function showCreateJobModal() {
    document.getElementById('create-job-modal').classList.add('active');
}

function closeCreateJobModal() {
    document.getElementById('create-job-modal').classList.remove('active');
}

async function createJob(event) {
    event.preventDefault();

    const jobType = document.getElementById('job-type').value;
    const jobValues = document.getElementById('job-values').value;
    const jobReward = document.getElementById('job-reward').value;
    const jobRequester = document.getElementById('job-requester').value;

    if (!jobType || !jobReward || !jobRequester) {
        showToast('Veuillez remplir tous les champs obligatoires', 'error');
        return;
    }

    try {
        const payload = {
            type: jobType,
            values: JSON.parse(jobValues || '[]'),
            reward: parseInt(jobReward),
            requester: jobRequester,
            nonce: Math.floor(Date.now() / 1000)
        };

        const response = await fetch(`${API_BASE_URL}/api/v1/compute/jobs`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(payload)
        });

        if (response.ok) {
            const data = await response.json();
            showToast('Job créé avec succès! ID: ' + data.id, 'success');
            closeCreateJobModal();
            loadJobs();
        } else {
            const error = await response.text();
            showToast('Erreur: ' + error, 'error');
        }
    } catch (error) {
        console.error('Failed to create job:', error);
        showToast('Erreur lors de la création du job', 'error');
    }
}

function claimJob(jobId) {
    showToast('Réclamation du job: ' + jobId, 'info');
    // In a full implementation, this would call the API
}

function viewJob(jobId) {
    showToast('Affichage du job: ' + jobId, 'info');
}

function loadMyJobs() {
    showToast('Chargement de vos jobs...', 'info');
    // In a full implementation, this would filter jobs by the connected wallet
}

// ============================================================================
// Validators
// ============================================================================
async function loadValidators() {
    if (!await checkNetworkStatus()) return;

    const validatorsBody = document.getElementById('validators-body');
    validatorsBody.innerHTML = '<tr class="loading-row"><td colspan="6">Chargement des validateurs...</td></tr>';

    try {
        const response = await fetch(`${API_BASE_URL}/api/v1/validators`);
        const validatorsData = await response.json();

        const validators = Array.isArray(validatorsData)
            ? validatorsData
            : (validatorsData.validators ?? []);

        if (!validators || validators.length === 0) {
            validatorsBody.innerHTML = '<tr><td colspan="6">Aucun validateur trouvé</td></tr>';
            return;
        }

        // Sort by stake (highest first)
        validators.sort((a, b) => (b.stake || b.Stake || 0) - (a.stake || a.Stake || 0));

        validatorsBody.innerHTML = validators.map((validator, index) => `
            <tr>
                <td>${index + 1}</td>
                <td class="hash">${validator.address || validator.Address || '-'}</td>
                <td>${formatNumber(validator.stake || validator.Stake || 0)}</td>
                <td>${formatNumber(validator.totalBalance || validator.TotalBalance || 0)}</td>
                <td>${formatNumber(validator.availableBalance || validator.AvailableBalance || 0)}</td>
                <td><span class="status-badge success">Registered</span></td>
            </tr>
        `).join('');

    } catch (error) {
        console.error('Failed to load validators:', error);
        validatorsBody.innerHTML = '<tr><td colspan="6">Erreur lors du chargement</td></tr>';
    }
}

function getUptimeStatus(validator) {
    // In a real implementation, we'd calculate uptime
    return 'success';
}

// ============================================================================
// Worker Info Modal
// ============================================================================
function showWorkerInfo() {
    document.getElementById('worker-info-modal').classList.add('active');
}

function closeWorkerInfoModal() {
    document.getElementById('worker-info-modal').classList.remove('active');
}

// ============================================================================
// Toast Notifications
// ============================================================================
function showToast(message, type = 'info') {
    // Remove existing toasts
    const existingToast = document.querySelector('.toast');
    if (existingToast) {
        existingToast.remove();
    }

    const toast = document.createElement('div');
    toast.className = `toast ${type} active`;
    toast.textContent = message;
    document.body.appendChild(toast);

    setTimeout(() => {
        toast.classList.remove('active');
        setTimeout(() => toast.remove(), 300);
    }, 3000);
}

// ============================================================================
// Utility Functions
// ============================================================================
function formatNumber(num) {
    if (typeof num !== 'number') return num || 0;
    return new Intl.NumberFormat('fr-FR').format(num);
}

function formatHash(hash) {
    if (!hash) return '-';
    return `${hash.substring(0, 8)}...${hash.substring(hash.length - 8)}`;
}

// ============================================================================
// Auto-refresh
// ============================================================================
function startAutoRefresh() {
    // Refresh dashboard every 10 seconds
    statsInterval = setInterval(() => {
        if (currentSection === 'dashboard') {
            loadDashboard();
        }
    }, 10000);

    // Refresh blocks every 15 seconds
    blocksInterval = setInterval(() => {
        if (currentSection === 'blocks') {
            loadBlocks();
        }
    }, 15000);

    // Refresh jobs every 10 seconds
    jobsInterval = setInterval(() => {
        if (currentSection === 'jobs') {
            loadJobs();
        }
    }, 10000);

    // Refresh validators every 30 seconds
    validatorsInterval = setInterval(() => {
        if (currentSection === 'validators') {
            loadValidators();
        }
    }, 30000);
}

function stopAutoRefresh() {
    clearInterval(statsInterval);
    clearInterval(blocksInterval);
    clearInterval(jobsInterval);
    clearInterval(validatorsInterval);
}

// ============================================================================
// Initialize
// ============================================================================
document.addEventListener('DOMContentLoaded', () => {
    initNavigation();
    checkNetworkStatus();
    loadDashboard();
    startAutoRefresh();

    // Close modals on outside click
    document.querySelectorAll('.modal').forEach(modal => {
        modal.addEventListener('click', (e) => {
            if (e.target === modal) {
                modal.classList.remove('active');
            }
        });
    });

    // Handle keyboard shortcuts
    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') {
            document.querySelectorAll('.modal').forEach(modal => {
                modal.classList.remove('active');
            });
        }
    });
});

// ============================================================================
// Docs
// ============================================================================
function showDocs() {
    window.open('https://github.com/Ulysse98/prism', '_blank');
}


// ============================================================================
// Recent activity - live Prism chain data
// ============================================================================
async function loadRecentActivity() {
    const chart = document.querySelector('.activity-chart');

    if (!chart) {
        return;
    }

    try {
        const response = await fetch(`${API_BASE_URL}/api/v1/work`);

        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`);
        }

        const data = await response.json();

        const entries = Array.isArray(data)
            ? data
            : (data.entries ?? []);

        if (entries.length === 0) {
            chart.innerHTML = `
                <div class="recent-activity-empty">
                    Aucune activité on-chain pour le moment.
                </div>
            `;
            return;
        }

        const recent = [...entries]
            .sort((a, b) => (b.block ?? 0) - (a.block ?? 0))
            .slice(0, 6);

        chart.innerHTML = `
            <div class="recent-activity-list">
                ${recent.map(entry => {
                    const proofId = entry.proofId || '-';

                    const shortProof = proofId.length > 20
                        ? `${proofId.slice(0, 10)}...${proofId.slice(-8)}`
                        : proofId;

                    const result =
                        Array.isArray(entry.resultValues) &&
                        entry.resultValues.length > 0
                            ? `[${entry.resultValues.join(', ')}]`
                            : entry.result;

                    return `
                        <div class="recent-activity-item">

                            <div class="recent-activity-icon">
                                ${entry.verified ? '&#10003;' : '!'}
                            </div>

                            <div class="recent-activity-main">

                                <div class="recent-activity-title">
                                    <strong>Block #${entry.block ?? '-'}</strong>

                                    <span class="recent-activity-badge ${
                                        entry.verified ? 'verified' : 'invalid'
                                    }">
                                        ${
                                            entry.verified
                                                ? 'PoUW vérifié'
                                                : 'Non vérifié'
                                        }
                                    </span>
                                </div>

                                <div class="recent-activity-meta">
                                    ${entry.worker || 'Worker inconnu'}
                                    &middot;
                                    ${entry.task || '-'}
                                    &middot;
                                    résultat ${result ?? '-'}
                                </div>

                                <div class="recent-activity-proof">
                                    Proof ${shortProof}
                                </div>

                            </div>

                            <div class="recent-activity-reward">
                                +${entry.reward ?? 0} PRISM
                                <span>score ${entry.score ?? 0}</span>
                            </div>

                        </div>
                    `;
                }).join('')}
            </div>
        `;

    } catch (error) {
        console.error('Failed to load recent activity:', error);

        chart.innerHTML = `
            <div class="recent-activity-empty">
                Impossible de charger l'activité récente.
            </div>
        `;
    }
}