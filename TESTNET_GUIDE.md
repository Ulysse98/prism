# Prism Testnet Deployment Guide

> **Guide complet pour dployer un testnet Prism public avec explorer et API**

---

## 0 Table des matikres

1. [Pr-requis](#1-pr-requis)
2. [Architecture du Testnet](#2-architecture-du-testnet)
3. [Dploiement avec Docker Compose](#3-dploiement-avec-docker-compose)
4. [Configuration](#4-configuration)
5. [Endpoints API](#5-endpoints-api)
6. [Utilisation de l'Explorer](#6-utilisation-de-l-explorer)
7. [Soumettre des Jobs ML](#7-soumettre-des-jobs-ml)
8. [Devenir un Worker](#8-devenir-un-worker)
9. [Dpannage](#9-dpannage)
10. [Mise  jour](#10-mise--jour)

---

## 1. Pr-requis

### Matriel
- **Systme d'exploitation** : Linux (Ubuntu 22.04+ recommand), macOS, ou Windows (WSL2)
- **Mmoire** : 8 Go minimum (16 Go recommand pour les jobs ML)
- **CPU** : 4 curs minimum
- **Stockage** : 50 Go SSD minimum
- **Ports** : 7001-7003 (P2P), 8080 (API), 80 (Explorer)

### Logiciels
- **Docker** : Version 20.10+ 
- **Docker Compose** : Version 2.0+
- **Git** : Pour cloner le dpt

### Vrification
```bash
# Vrifier Docker
docker --version
docker compose version

# Vrifier Git
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
|  | Worker     |  (Optionnel - pour excuter des jobs ML)          |
|  | (Python)   |                                                       |
|  +------------+                                                       |
|                                                                  |
+------------------------------------------------------------------+
```

### Composants
| Composant | Port | Rle |
|-----------|------|-------|
| **Node 1** | 7001 | Nud principal (bootstrap) |
| **Node 2** | 7002 | Nud pair |
| **Node 3** | 7003 | Nud pair |
| **API** | 8080 | Serveur REST HTTP |
| **Explorer** | 80 | Interface web |
| **Worker** | - | Excuteur de jobs ML (optionnel) |

---

## 3. Dploiement avec Docker Compose

### 0 Cloner le dpt
```bash
git clone https://github.com/Ulysse98/prism.git
cd prism
```

### 1 Construire les images
```bash
# Construire l'image du node Prism
docker build -t prism-node:v0.48.0 .
```

### 2 Dmarrer le testnet
```bash
# Dmarrer tous les services (nodes + API + explorer)
docker compose -f docker-compose.testnet.yaml up -d
```

### 3 Vrifier le dploiement
```bash
# Voir l'tat des conteneurs
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

### 4 Accder aux services
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
Le fichier `docker-compose.testnet.yaml` contient une configuration par dfaut. Vous pouvez personnaliser :

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

### Configuration avance

#### Changer le nombre de nodes
Modifiez le fichier `docker-compose.testnet.yaml` pour ajouter ou supprimer des nodes :

```yaml
services:
  node-4:
    image: prism-node:v0.48.0
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
| GET | `/api/v1/health` | Vrifie la sant du node |
| GET | `/api/v1/status` | tat du rseau |

**Exemple :**
```bash
curl http://localhost:8080/api/v1/health
```

**Rponse :**
```json
{
  "status": "ok",
  "network": "Prism",
  "version": "0.48.0",
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

**Rponse :**
```json
{
  "network": "Prism",
  "version": "0.48.0",
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

**Rponse :**
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
| GET | `/api/v1/compute/jobs/{id}` | Dtails d'un job |
| POST | `/api/v1/compute/jobs` | Cre un nouveau job |
| POST | `/api/v1/compute/jobs/{id}/claim` | Rclame un job |
| POST | `/api/v1/compute/jobs/{id}/complete` | Complte un job |

**Exemple - Crer un job :**
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

**Rponse :**
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

**Exemple - Rclamer un job :**
```bash
curl -X POST http://localhost:8080/api/v1/compute/jobs/job_abc123/claim \
  -H "Content-Type: application/json" \
  -d '{
    "worker": "Bob",
    "publicKey": "pub_123456"
  }'
```

**Exemple - Complter un job :**
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
| GET | `/api/v1/work` | Historique des t ches de calcul |

### Humanity Verification
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/humanity` | Vrifications humanity |

### Reserved Transfers
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/reserved` | Transfers rservs |

### Mining
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/mine/tasks` | T ches de minage |
| POST | `/api/v1/mine/start` | Dmarrer le minage |
| POST | `/api/v1/mine/submit` | Soumettre une preuve |

### Cross-chain Settlements
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/settlements` | Liste des settlements |
| GET | `/api/v1/settlements/{registryId}` | Dtails d'un settlement |

### Receipts
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/receipts/{jobId}` | Rçu d'un job |

---

## 6. Utilisation de l'Explorer

### Accs
Ouvrez votre navigateur et allez sur [http://localhost](http://localhost) (ou le port que vous avez configur).

### Fonctionnalits

#### Dashboard
- **Block Height** : Hauteur actuelle de la blockchain
- **Validators** : Nombre de validateurs actifs
- **Total Stake** : Montant total de PRISM stak
- **ML Jobs** : Nombre de jobs de calcul en cours
- **Chain Hash** : Hash du dernier block
- **Protocol** : Version du protocole

#### Blocks
- Liste des derniers blocks
- Recherche de blocks par hash ou hauteur
- Dtails de chaque block (proposer, transactions, useful work)

#### ML Jobs
- Liste de tous les jobs de calcul
- Filtres par statut (OPEN, CLAIMED, VERIFIED)
- Cration de nouveaux jobs
- Rclamation de jobs disponibles

#### Validators
- Liste des validateurs
- Tri par stake (du plus lev au plus bas)
- Affichage des balances (stake, total, available)

#### Compute
- Documentation de l'API Compute
- Exemples d'utilisation
- Configuration des workers

---

## 7. Soumettre des Jobs ML

### Via l'Explorer
1. Allez dans la section **Compute** ou **ML Jobs**
2. Cliquez sur **Crer un Job**
3. Remplissez le formulaire :
   - **Type de t che** : Slectionnez le type (sum_squares, dot_product, prime_count, matrix_multiply, ml_inference_quantized)
   - **Valeurs** : Entrez les valeurs au format JSON (ex: `[1, 2, 3, 4, 5]`)
   - **Rcompense** : Montant en PRISM pour le worker
   - **Requester** : Votre adresse ou nom
4. Cliquez sur **Crr le Job**

### Via l'API
Voir la section [Endpoints API - Compute Jobs](#compute-jobs-ml-tasks).

### Types de jobs supports

| Type | Description | Exemple de valeurs |
|------|-------------|-------------------|
| `sum_squares` | Somme des carrs | `[1, 2, 3, 4, 5]` |
| `dot_product` | Produit scalaire | `{"values": [1,2,3], "valuesB": [4,5,6]}` |
| `prime_count` | Compter les nombres premiers | `[1, 2, 3, 4, 5, 6, 7, 8, 9, 10]` |
| `matrix_multiply` | Multiplication matricielle | `{"rowsA": 2, "colsA": 3, "colsB": 2, "values": [...], "valuesB": [...]}` |
| `ml_inference_quantized` | Infrence ML quantifie | Voir [Python Runtime](#python-runtime) |

---

## 8. Devenir un Worker

### Pr-requis
- Un **wallet Prism** avec des fonds (pour les frais de transaction)
- Une **cl publique/prive** pour signer les preuves
- **Python 3.10+** (pour le runtime ML)

### Configuration

#### 0 Crer un wallet
```bash
# Dans le conteneur du node
docker exec -it prism-node-1 prism wallets
```

#### 1 Installer les dpendances Python
```bash
# Crer un environnement virtuel
python -m venv .venv-prism
source .venv-prism/bin/activate  # Linux/macOS
# .\.venv-prism\Scripts\activate  # Windows

# Installer les dpendances
pip install -r tools/python/requirements-worker.txt
```

#### 2 Configurer le worker
Crez un fichier `worker-config.json` :
```json
{
  "api_url": "http://localhost:8080",
  "wallet_path": "/path/to/wallet.json",
  "private_key_path": "/path/to/private_key.pem",
  "poll_interval": 5,
  "max_concurrent_jobs": 10
}
```

#### 3 Lancer le worker
```bash
# Depuis le dpt
python tools/python/prism_worker.py --config worker-config.json
```

### Fonctionnement
1. Le worker **poll** l'API pour de nouveaux jobs (toutes les 5 secondes)
2. Il **rcupre** un job OPEN
3. Il **excute** le calcul
4. Il **signe** la preuve avec sa cl prive
5. Il **soumet** la preuve via l'API
6. Il **reoit** la rcompense en PRISM

### Docker Worker (Optionnel)
Vous pouvez aussi lancer un worker dans un conteneur Docker :

```bash
# Construire l'image du worker
docker build -t prism-worker:v0.48.0 -f Dockerfile.worker .

# Lancer le worker
docker run -d \
  --name prism-worker \
  --network prism-testnet \
  -v $(pwd)/worker-data:/app/data \
  -e PRISM_API=http://prism-api:8080 \
  prism-worker:v0.48.0
```

---

## 9. Dpannage

### Problmes courants

#### Les nodes ne dmarrent pas
```bash
# Vrifier les logs
docker logs prism-node-1
docker logs prism-node-2
docker logs prism-node-3

# Vrifier que les ports sont disponibles
netstat -tuln | grep 700
```

**Solutions :**
- Vrifier que Docker est en cours d'excution
- Vrifier que les ports ne sont pas dj utiliss
- Vrifier les permissions sur les volumes

#### L'API ne rpond pas
```bash
# Vrifier que l'API est en cours d'excution
docker logs prism-api

# Tester l'API directement
curl http://localhost:8080/api/v1/health
```

**Solutions :**
- Vrifier que le node 1 est en cours d'excution
- Vrifier que le chemin du data directory est correct
- Vrifier les permissions sur les fichiers

#### L'explorer ne charge pas
```bash
# Vrifier que NGINX est en cours d'excution
docker logs prism-explorer

# Tester NGINX directement
curl http://localhost
```

**Solutions :**
- Vrifier que les fichiers de l'explorer sont prsents dans `web/explorer/`
- Vrifier que NGINX a les permissions pour lire les fichiers

#### Les nodes ne se connectent pas entre eux
```bash
# Vrifier la connectivit entre les conteneurs
docker exec -it prism-node-2 ping prism-node-1

# Vrifier les logs de synchronisation
docker logs prism-node-2 | grep -i sync
```

**Solutions :**
- Vrifier que tous les nodes utilisent le mme **Chain ID**
- Vrifier que les nodes sont sur le mme rseau Docker
- Redmarrer les nodes avec `--peer` pointant vers le node 1

### Commandes utiles

```bash
# Redmarrer tous les services
docker compose -f docker-compose.testnet.yaml restart

# Arrter tous les services
docker compose -f docker-compose.testnet.yaml down

# Supprimer les volumes (ATTENTION: cela supprime toutes les donnes!)
docker compose -f docker-compose.testnet.yaml down -v

# Voir les logs en temps rel
docker compose -f docker-compose.testnet.yaml logs -f

# Voir les logs d'un service spcifique
docker compose -f docker-compose.testnet.yaml logs -f node-1

# Excuter une commande dans un conteneur
docker exec -it prism-node-1 prism status
```

---

## 10. Mise  jour

### Mettre  jour le code
```bash
# Dans le dpt
git pull origin master

# Reconstruire les images
docker build -t prism-node:v0.48.0 .

# Redmarrer les services
docker compose -f docker-compose.testnet.yaml down
docker compose -f docker-compose.testnet.yaml up -d
```

### Mettre  jour la version
Modifiez la version dans :
- `cmd/prism/main.go` (constante `prismVersion`)
- `docker-compose.testnet.yaml` (tag de l'image)
- `Dockerfile` (si ncessaire)

---

## Annexes

### A. Python Runtime

Le runtime Python permet d'excuter des t ches ML de manire dterministe. Voir :
- `tools/python/README.md`
- `tools/python/WORKER.md`
- `tools/python/WATCH.md`

### B. Configuration avance

#### Changer la configuration de la chain
Vous pouvez personnaliser la configuration de la blockchain en modifiant les fichiers dans `internal/blockchain/`.

#### Changer les rcompenses
Les rcompenses sont configures dans :
- `internal/consensus/useful_work_reward.go`
- `internal/consensus/pool_reward.go`

#### Changer les limites
Les limites (max jobs, max stake, etc.) sont configures dans :
- `internal/compute/marketplace.go`
- `internal/blockchain/blockchain.go`

---

## Support

- **GitHub** : [https://github.com/Ulysse98/prism](https://github.com/Ulysse98/prism)
- **Issues** : Ouvrez une issue sur GitHub pour signaler un bug
- **Contributions** : Les pull requests sont les bienvenues !

---

*Dernire mise  jour : v0.48.0*
