# Prism Testnet Deployment Guide

> **Guide complet pour deployer un testnet Prism public avec explorer et API**

---

## 0 Table des matieres

1. [Pre-requis](#1-pre-requis)
2. [Architecture du Testnet](#2-architecture-du-testnet)
3. [Deploiement avec Docker Compose](#3-deploiement-avec-docker-compose)
4. [Configuration](#4-configuration)
5. [Endpoints API](#5-endpoints-api)
6. [Utilisation de l'Explorer](#6-utilisation-de-l-explorer)
7. [Soumettre des Jobs ML](#7-soumettre-des-jobs-ml)
8. [Devenir un Worker](#8-devenir-un-worker)
9. [Depannage](#9-depannage)
10. [Mise e jour](#10-mise-e-jour)

---

## 1. Pre-requis

### Materiel
- **Systeme d'exploitation** : Linux (Ubuntu 22.04+ recommande), macOS, ou Windows (WSL2)
- **Memoire** : 8 Go minimum (16 Go recommande pour les jobs ML)
- **CPU** : 4 coeurs minimum
- **Stockage** : 50 Go SSD minimum
- **Ports** : 7001-7003 (P2P), 8080 (API), 80 (Explorer)

### Logiciels
- **Docker** : Version 20.10+ 
- **Docker Compose** : Version 2.0+
- **Git** : Pour cloner le depot

### Verification
```bash
# Verifier Docker
docker --version
docker compose version

# Verifier Git
git --version
```

---

## 2. Architecture du Testnet

```
+------------------------------------------------------------------+
|                    PRISM TESTNET ARCHITECTURE                     |
+------------------------------------------------------------------+
|                                                                  |
|  +------------+     +------------+     +------------+              |
|  | Node 1     |     | Node 2     |     | Node 3     |              |
|  | (P2P:7001) |<--->| (P2P:7002) |<--->| (P2P:7003) |              |
|  +------------+     +------------+     +------------+              |
|                                                                  |
|  +------------+     +-------------------+                              |
|  | API Server |     | NGINX + Explorer |                              |
|  | (HTTP:8080)|     | (HTTP:80)        |                              |
|  +------------+     +-------------------+                              |
|                                                                  |
|  +------------+                                                       |
|  | Worker     |  (Optionnel - pour executer des jobs ML)          |
|  | (Python)   |                                                       |
|  +------------+                                                       |
|                                                                  |
+------------------------------------------------------------------+
```

### Composants
| Composant | Port | Role |
|-----------|------|-------|
| **Node 1** | 7001 | Noeud principal (bootstrap) |
| **Node 2** | 7002 | Noeud pair |
| **Node 3** | 7003 | Noeud pair |
| **API** | 8080 | Serveur REST HTTP |
| **Explorer** | 80 | Interface web |
| **Worker** | - | Executeur de jobs ML (optionnel) |

---

## 3. Deploiement avec Docker Compose

### 0 Cloner le depot
```bash
git clone https://github.com/Ulysse98/prism.git
cd prism
```

### 1 Construire les images
```bash
# Construire l'image du node Prism
docker build -t prism-node:v0.50.0 .
```

### 2 Demarrer le testnet
```bash
# Demarrer tous les services (nodes + API + explorer)
docker compose -f docker-compose.testnet.yaml up -d
```

### 3 Verifier le deploiement
```bash
# Voir l'etat des conteneurs
docker ps

# Voir les logs des nodes
docker logs prism-node-1
docker logs prism-node-2
docker logs prism-node-3

# Voir les logs de l'API
docker logs prism-api

# Voir les logs de l'explorer
docker logs prism-explorer
```

### 4 Acceder aux services
| Service | URL | Description |
|---------|-----|-------------|
| **Explorer** | [http://localhost](http://localhost) | Interface web |
| **API** | [http://localhost:8080](http://localhost:8080) | REST API |
| **Node 1** | localhost:7001 | P2P Node |
| **Node 2** | localhost:7002 | P2P Node |
| **Node 3** | localhost:7003 | P2P Node |

---

## 4. Configuration

### Configuration de base
Le fichier `docker-compose.testnet.yaml` contient une configuration par defaut. Vous pouvez personnaliser :

```yaml
# Exemple de personnalisation
services:
  node-1:
    ports:
      - "7001:7001"
    environment:
      - PRISM_ENV=testnet
      - PRISM_LOG_LEVEL=info
  
  api:
    ports:
      - "8080:8080"
    environment:
      - PRISM_API_RATE_LIMIT=100
  
  explorer:
    ports:
      - "80:80"
```

### Configuration avancee

#### Changer le nombre de nodes
Modifiez le fichier `docker-compose.testnet.yaml` pour ajouter ou supprimer des nodes :

```yaml
services:
  node-4:
    image: prism-node:v0.50.0
    container_name: prism-node-4
    command:
      - node
      - --host
      - 0.0.0.0
      - --port
      - "7004"
      - --peer
      - prism-node-1:7001
    ports:
      - "7004:7004"
    volumes:
      - node4-data:/app/data
    networks:
      - prism-testnet
```

#### Changer les ports
```yaml
services:
  api:
    ports:
      - "3000:8080"  # API accessible sur le port 3000
  explorer:
    ports:
      - "3001:80"    # Explorer accessible sur le port 3001
```

---

## 5. Endpoints API

### Health & Status
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/health` | Verifie la sante du node |
| GET | `/api/v1/status` | otat du reseau |

**Exemple :**
```bash
curl http://localhost:8080/api/v1/health
```

**Reponse :**
```json
{
  "status": "ok",
  "network": "Prism",
  "version": "0.50.0",
  "protocol": "0.36",
  "chainId": "prism-testnet",
  "height": 12345,
  "chainValid": true
}
```

### Blockchain
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/status` | Informations sur la blockchain |

**Exemple :**
```bash
curl http://localhost:8080/api/v1/status
```

**Reponse :**
```json
{
  "network": "Prism",
  "version": "0.50.0",
  "protocol": "0.36",
  "chainId": "prism-testnet",
  "height": 12345,
  "blocks": 12345,
  "validators": 3,
  "totalStake": 1000000,
  "totalSupply": 10000000,
  "chainValid": true,
  "genesisHash": "a1b2c3...",
  "lastHash": "d4e5f6..."
}
```

### Validators
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/validators` | Liste des validateurs |

**Exemple :**
```bash
curl http://localhost:8080/api/v1/validators
```

**Reponse :**
```json
[
  {
    "name": "Node 1",
    "address": "0x1234...",
    "stake": 500000,
    "totalBalance": 600000,
    "availableBalance": 100000
  },
  {
    "name": "Node 2",
    "address": "0x5678...",
    "stake": 300000,
    "totalBalance": 400000,
    "availableBalance": 100000
  }
]
```

### Participation (PoUP)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/participation` | Scores de participation |

### Compute Jobs (ML Tasks)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/compute/jobs` | Liste tous les jobs |
| GET | `/api/v1/compute/jobs/{id}` | Details d'un job |
| POST | `/api/v1/compute/jobs` | Cree un nouveau job |
| POST | `/api/v1/compute/jobs/{id}/claim` | Reclame un job |
| POST | `/api/v1/compute/jobs/{id}/complete` | Complete un job |

**Exemple - Creer un job :**
```bash
curl -X POST http://localhost:8080/api/v1/compute/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "type": "sum_squares",
    "values": [1, 2, 3, 4, 5],
    "reward": 100,
    "requester": "Alice"
  }'
```

**Reponse :**
```json
{
  "id": "job_abc123",
  "type": "sum_squares",
  "values": [1, 2, 3, 4, 5],
  "reward": 100,
  "requester": "Alice",
  "status": "OPEN",
  "createdAt": 1234567890
}
```

**Exemple - Reclamer un job :**
```bash
curl -X POST http://localhost:8080/api/v1/compute/jobs/job_abc123/claim \
  -H "Content-Type: application/json" \
  -d '{
    "worker": "Bob",
    "publicKey": "pub_123456"
  }'
```

**Exemple - Completer un job :**
```bash
curl -X POST http://localhost:8080/api/v1/compute/jobs/job_abc123/complete \
  -H "Content-Type: application/json" \
  -d '{
    "proof": {
      "id": "proof_123",
      "taskId": "job_abc123",
      "worker": "Bob",
      "result": 55,
      "outputHash": "hash_abc123",
      "signature": "sig_abc123"
    }
  }'
```

### Useful Work
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/work` | Historique des taches de calcul |

### Humanity Verification
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/humanity` | Verifications humanity |

### Reserved Transfers
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/reserved` | Transfers reserves |

### Mining
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/mine/tasks` | Taches de minage |
| POST | `/api/v1/mine/start` | Demarrer le minage |
| POST | `/api/v1/mine/submit` | Soumettre une preuve |

### Cross-chain Settlements
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/settlements` | Liste des settlements |
| GET | `/api/v1/settlements/{registryId}` | Details d'un settlement |

### Receipts
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/receipts/{jobId}` | Reçu d'un job |

---

## 6. Utilisation de l'Explorer

### Acces
Ouvrez votre navigateur et allez sur [http://localhost](http://localhost) (ou le port que vous avez configure).

### Fonctionnalites

#### Dashboard
- **Block Height** : Hauteur actuelle de la blockchain
- **Validators** : Nombre de validateurs actifs
- **Total Stake** : Montant total de PRISM stake
- **ML Jobs** : Nombre de jobs de calcul en cours
- **Chain Hash** : Hash du dernier block
- **Protocol** : Version du protocole

#### Blocks
- Liste des derniers blocks
- Recherche de blocks par hash ou hauteur
- Details de chaque block (proposer, transactions, useful work)

#### ML Jobs
- Liste de tous les jobs de calcul
- Filtres par statut (OPEN, CLAIMED, VERIFIED)
- Creation de nouveaux jobs
- Reclamation de jobs disponibles

#### Validators
- Liste des validateurs
- Tri par stake (du plus eleve au plus bas)
- Affichage des balances (stake, total, available)

#### Compute
- Documentation de l'API Compute
- Exemples d'utilisation
- Configuration des workers

---

## 7. Soumettre des Jobs ML

### Via l'Explorer
1. Allez dans la section **Compute** ou **ML Jobs**
2. Cliquez sur **Creer un Job**
3. Remplissez le formulaire :
   - **Type de tache** : Selectionnez le type (sum_squares, dot_product, prime_count, matrix_multiply, ml_inference_quantized)
   - **Valeurs** : Entreez les valeurs au format JSON (ex: `[1, 2, 3, 4, 5]`)
   - **Recompense** : Montant en PRISM pour le worker
   - **Requester** : Votre adresse ou nom
4. Cliquez sur **Crer le Job**

### Via l'API
Voir la section [Endpoints API - Compute Jobs](#compute-jobs-ml-tasks).

### Types de jobs supportes

| Type | Description | Exemple de valeurs |
|------|-------------|-------------------|
| `sum_squares` | Somme des carres | `[1, 2, 3, 4, 5]` |
| `dot_product` | Produit scalaire | `{"values": [1,2,3], "valuesB": [4,5,6]}` |
| `prime_count` | Compter les nombres premiers | `[1, 2, 3, 4, 5, 6, 7, 8, 9, 10]` |
| `matrix_multiply` | Multiplication matricielle | `{"rowsA": 2, "colsA": 3, "colsB": 2, "values": [...], "valuesB": [...]}` |
| `ml_inference_quantized` | Inference ML quantifiee | Voir [Python Runtime](#python-runtime) |

---

## 8. Devenir un Worker

### Pre-requis
- Un **wallet Prism** avec des fonds (pour les frais de transaction)
- Une **cle publique/privee** pour signer les preuves
- **Python 3.10+** (pour le runtime ML)

### Configuration

#### 0 Creer un wallet
```bash
# Dans le conteneur du node
docker exec -it prism-node-1 prism wallets
```

#### 1 Installer les dependances Python
```bash
# Creer un environnement virtuel
python -m venv .venv-prism
source .venv-prism/bin/activate  # Linux/macOS
# .\.venv-prism\Scripts\activate  # Windows

# Installer les dependances
pip install -r tools/python/requirements-worker.txt
```

#### 2 Lancer le worker

Le worker utilise les wallets Prism stockes dans le data directory du node.
Les options globales `--api`, `--data` et `--timeout` doivent etre placees
avant la sous-commande.

Exemple avec Bob en mode automatique :

```bash
python tools/python/prism_ml_worker.py \
  --api http://127.0.0.1:8080/api/v1 \
  --data data \
  watch \
  --worker Bob \
  --poll-interval 5
```

Pour effectuer un seul passage sur la file puis quitter :

```bash
python tools/python/prism_ml_worker.py \
  --api http://127.0.0.1:8080/api/v1 \
  --data data \
  watch \
  --worker Bob \
  --once
```
### Fonctionnement
1. Le worker **poll** l'API pour de nouveaux jobs (toutes les 5 secondes)
2. Il **recupere** un job OPEN
3. Il **execute** le calcul
4. Il **signe** la preuve avec sa cle privee
5. Il **soumet** la preuve via l'API
6. Il **recoit** la recompense en PRISM

### Docker Worker (Optionnel)

Le profil `worker` lance le worker Python automatique avec le wallet Bob :

```bash
docker build -t prism-worker:v0.50.0 -f Dockerfile.worker .

docker compose \
  -f docker-compose.testnet.yaml \
  --profile worker \
  up -d worker

docker logs -f prism-worker
```
---

## 9. Depannage

### Problemes courants

#### Les nodes ne demarrent pas
```bash
# Verifier les logs
docker logs prism-node-1
docker logs prism-node-2
docker logs prism-node-3

# Verifier que les ports sont disponibles
netstat -tuln | grep 700
```

**Solutions :**
- Verifier que Docker est en cours d'execution
- Verifier que les ports ne sont pas deja utilises
- Verifier les permissions sur les volumes

#### L'API ne repond pas
```bash
# Verifier que l'API est en cours d'execution
docker logs prism-api

# Tester l'API directement
curl http://localhost:8080/api/v1/health
```

**Solutions :**
- Verifier que le node 1 est en cours d'execution
- Verifier que le chemin du data directory est correct
- Verifier les permissions sur les fichiers

#### L'explorer ne charge pas
```bash
# Verifier que NGINX est en cours d'execution
docker logs prism-explorer

# Tester NGINX directement
curl http://localhost
```

**Solutions :**
- Verifier que les fichiers de l'explorer sont presents dans `web/explorer/`
- Verifier que NGINX a les permissions pour lire les fichiers

#### Les nodes ne se connectent pas entre eux
```bash
# Verifier la connectivite entre les conteneurs
docker exec -it prism-node-2 ping prism-node-1

# Verifier les logs de synchronisation
docker logs prism-node-2 | grep -i sync
```

**Solutions :**
- Verifier que tous les nodes utilisent le meme **Chain ID**
- Verifier que les nodes sont sur le meme reseau Docker
- Redemarrer les nodes avec `--peer` pointant vers le node 1

### Commandes utiles

```bash
# Redemarrer tous les services
docker compose -f docker-compose.testnet.yaml restart

# Arreter tous les services
docker compose -f docker-compose.testnet.yaml down

# Supprimer les volumes (ATTENTION: cela supprime toutes les donnees!)
docker compose -f docker-compose.testnet.yaml down -v

# Voir les logs en temps reel
docker compose -f docker-compose.testnet.yaml logs -f

# Voir les logs d'un service specifique
docker compose -f docker-compose.testnet.yaml logs -f node-1

# Executer une commande dans un conteneur
docker exec -it prism-node-1 prism status
```

---

## 10. Mise e jour

### Mettre e jour le code
```bash
# Dans le depot
git pull origin master

# Reconstruire les images
docker build -t prism-node:v0.50.0 .

# Redemarrer les services
docker compose -f docker-compose.testnet.yaml down
docker compose -f docker-compose.testnet.yaml up -d
```

### Mettre e jour la version
Modifiez la version dans :
- `cmd/prism/main.go` (constante `prismVersion`)
- `docker-compose.testnet.yaml` (tag de l'image)
- `Dockerfile` (si necessaire)

---

## Annexes

### A. Python Runtime

Le runtime Python permet d'executer des taches ML de maniere deterministe. Voir :
- `tools/python/README.md`
- `tools/python/WORKER.md`
- `tools/python/WATCH.md`

### B. Configuration avancee

#### Changer la configuration de la chain
Vous pouvez personnaliser la configuration de la blockchain en modifiant les fichiers dans `internal/blockchain/`.

#### Changer les recompenses
Les recompenses sont configurees dans :
- `internal/consensus/useful_work_reward.go`
- `internal/consensus/pool_reward.go`

#### Changer les limites
Les limites (max jobs, max stake, etc.) sont configurees dans :
- `internal/compute/marketplace.go`
- `internal/blockchain/blockchain.go`

---

## Support

- **GitHub** : [https://github.com/Ulysse98/prism](https://github.com/Ulysse98/prism)
- **Issues** : Ouvrez une issue sur GitHub pour signaler un bug
- **Contributions** : Les pull requests sont les bienvenues !

---

*Derniere mise e jour : v0.50.0*
