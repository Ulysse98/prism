#!/bin/bash

# ============================================================================
# Prism Testnet Initialization Script
# 
# This script initializes a fresh Prism testnet with:
# - Genesis block with initial balances
# - Multiple nodes with persistent state
# - API server
# - Web explorer
# ============================================================================

set -euo pipefail

# ============================================================================
# Configuration
# ============================================================================

# Version
PRISM_VERSION="v0.50.0"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# ============================================================================
# Functions
# ============================================================================

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# ============================================================================
# Main
# ============================================================================

main() {
    local command="${1:-help}"
    
    case "$command" in
        start)
            start_testnet
            ;;
        stop)
            stop_testnet
            ;;
        restart)
            stop_testnet
            start_testnet
            ;;
        clean)
            clean_testnet
            ;;
        status)
            status_testnet
            ;;
        logs)
            show_logs "${2:-}"
            ;;
        build)
            build_images
            ;;
        init)
            init_genesis
            ;;
        help|--help|-h|"")
            show_help
            ;;
        *)
            log_error "Commande inconnue: $command"
            show_help
            exit 1
            ;;
    esac
}

# ============================================================================
# Help
# ============================================================================

show_help() {
    cat << EOF

${BLUE}Prism Testnet Initialization Script${NC}

Usage: ./init-testnet.sh <command> [options]

Commands:
  start       Demarre le testnet complet (nodes + API + explorer)
  stop        Arrete tous les services
  restart     Redemarre le testnet
  clean       Supprime tous les conteneurs, images et volumes
  status      Affiche l'etat des services
  logs [service]  Affiche les logs (tous ou service specifique)
  build       Construit les images Docker
  init        Initialise la configuration genesis
  help        Affiche cette aide

Examples:
  ./init-testnet.sh start
  ./init-testnet.sh stop
  ./init-testnet.sh logs node-1
  ./init-testnet.sh clean

EOF
}

# ============================================================================
# Build Images
# ============================================================================

build_images() {
    log_info "Construction de l'image Prism node..."
    docker build -t prism-node:$PRISM_VERSION .
    log_success "Image prism-node:$PRISM_VERSION construite"
    
    log_info "Construction de l'image Prism worker..."
    docker build -t prism-worker:$PRISM_VERSION -f Dockerfile.worker .
    log_success "Image prism-worker:$PRISM_VERSION construite"
}

# ============================================================================
# Initialize Genesis
# ============================================================================

init_genesis() {
    log_info "Initialisation de la configuration genesis..."
    
    # Create data directories
    mkdir -p data/node-7001 data/node-7002 data/node-7003
    
    log_success "Configuration genesis initialisee"
}

# ============================================================================
# Start Testnet
# ============================================================================

start_testnet() {
    log_info "Demarrage du testnet Prism..."
    
    # Check if images exist
    if ! docker image inspect prism-node:$PRISM_VERSION &>/dev/null; then
        log_warning "Image prism-node:$PRISM_VERSION non trouvee, construction..."
        build_images
    fi
    
    # Initialize genesis if not exists
    if [ ! -d "data/node-7001" ]; then
        init_genesis
    fi
    
    # Start the testnet
    log_info "Lancement des conteneurs..."
    docker compose -f docker-compose.testnet.yaml up -d
    
    # Wait for nodes to start
    log_info "Attente du demarrage des nodes..."
    sleep 5
    
    # Check health
    log_info "Verification de l'etat des services..."
    
    local max_attempts=30
    local attempt=1
    local all_healthy=false
    
    while [ $attempt -le $max_attempts ] && [ "$all_healthy" = false ]; do
        if check_api_health; then
            all_healthy=true
            break
        fi
        sleep 2
        attempt=$((attempt + 1))
    done
    
    if [ "$all_healthy" = true ]; then
        log_success "Testnet Prism demarre avec succes!"
        show_status
    else
        log_error "Impossible de demarrer le testnet apres $max_attempts tentatives"
        exit 1
    fi
}

# ============================================================================
# Stop Testnet
# ============================================================================

stop_testnet() {
    log_info "Arret des services..."
    docker compose -f docker-compose.testnet.yaml down
    log_success "Services arretes"
}

# ============================================================================
# Clean Testnet
# ============================================================================

clean_testnet() {
    log_warning "Suppression de tous les conteneurs, images et volumes Prism..."
    
    # Stop all services
    docker compose -f docker-compose.testnet.yaml down 2>/dev/null || true
    
    # Remove volumes
    docker volume rm prism-devnet-node1-data 2>/dev/null || true
    docker volume rm prism-devnet-node2-data 2>/dev/null || true
    docker volume rm prism-devnet-node3-data 2>/dev/null || true
    
    # Remove images
    docker rmi prism-node:$PRISM_VERSION 2>/dev/null || true
    docker rmi prism-worker:$PRISM_VERSION 2>/dev/null || true
    
    # Remove local data
    rm -rf data/
    
    log_success "Nettoyage termine"
}

# ============================================================================
# Status
# ============================================================================

status_testnet() {
    show_status
}

show_status() {
    echo ""
    echo "${BLUE}=== Prism Testnet Status ===${NC}"
    echo ""
    
    # Show containers
    echo "${BLUE}Conteneurs:${NC}"
    docker ps --filter "name=prism" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo "Aucun conteneur en cours d'execution"
    echo ""
    
    # Check API health
    if check_api_health; then
        echo "${GREEN}API: Healthy${NC}"
    else
        echo "${RED}API: Unhealthy${NC}"
    fi
    
    # Show network info
    if check_api_health; then
        local status=$(curl -s http://localhost:8080/api/v1/status)
        if [ -n "$status" ]; then
            local height=$(echo "$status" | jq -r '.height // "-"')
            local validators=$(echo "$status" | jq -r '.validators // "-"')
            local chain_valid=$(echo "$status" | jq -r '.chainValid // "-"')
            
            echo ""
            echo "${BLUE}Network Info:${NC}"
            echo "  Height: $height"
            echo "  Validators: $validators"
            echo "  Chain Valid: $chain_valid"
        fi
    fi
    
    echo ""
    echo "${BLUE}URLs:${NC}"
    echo "  Explorer: http://localhost"
    echo "  API: http://localhost:8080"
    echo ""
}

# ============================================================================
# Logs
# ============================================================================

show_logs() {
    local service="${1:-}"
    
    if [ -z "$service" ]; then
        echo "${BLUE}=== All Prism Services Logs ===${NC}"
        docker compose -f docker-compose.testnet.yaml logs -f
    else
        echo "${BLUE}=== Logs for $service ===${NC}"
        docker compose -f docker-compose.testnet.yaml logs -f "$service"
    fi
}

# ============================================================================
# Health Checks
# ============================================================================

check_api_health() {
    if ! command -v curl &>/dev/null; then
        log_error "curl non installe"
        return 1
    fi
    
    if curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/api/v1/health | grep -q "200"; then
        return 0
    fi
    return 1
}

# ============================================================================
# Run
# ============================================================================

main "$@"
