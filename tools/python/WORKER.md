# Prism v0.37 : worker Python HTTP signe

Extension du paquet `prism-python-v037.zip` deja installe et valide :
126 comparaisons numeriques Python/Go reussies sur le depot utilisateur.
Cette archive ajoute sept fichiers dans `tools/python/` et ne remplace aucun
fichier du premier paquet. Elle ne modifie aucun fichier Go du depot.

## Installation et verification sous PowerShell

Enregistrer `prism-python-worker-v037.zip` dans Downloads. Le premier paquet
doit deja etre extrait dans le depot. Lancer ce bloc :

```powershell
& {
    $ErrorActionPreference = "Stop"
    Set-Location C:\Users\ulyss\prism

    Expand-Archive -LiteralPath "$env:USERPROFILE\Downloads\prism-python-worker-v037.zip" -DestinationPath .

    py -m venv .venv-prism-ml
    if ($LASTEXITCODE -ne 0) { throw "Creation de l'environnement Python en echec." }
    Set-Content -LiteralPath .\.venv-prism-ml\.gitignore -Value '*' -Encoding ascii
    $prismPy = ".\.venv-prism-ml\Scripts\python.exe"

    & $prismPy -m pip install -r .\tools\python\requirements-worker.txt
    if ($LASTEXITCODE -ne 0) { throw "Installation en echec." }

    & $prismPy -m unittest discover -s tools/python -v
    if ($LASTEXITCODE -ne 0) { throw "Tests Python en echec." }

    & $prismPy .\tools\python\check_go_proofs.py --repo .
    if ($LASTEXITCODE -ne 0) { throw "Verification des preuves par Go en echec." }
}
```

Le comparateur Go attendu annonce 16 cas : six preuves signees identiques
a celles de `usefulwork.Execute` et dix falsifications rejetees. Pour les six
preuves valides, il appelle aussi `compute.NewJob`, `Claim` et `Complete`.
Il utilise exclusivement une cle de test publique et deterministe, jamais les
wallets utilisateur. Le programme Go temporaire est supprime a la fin.

La signature utilise le module Ed25519 de `cryptography`, avec la seed de
32 octets extraite du format Go `seed || public key` de 64 octets. Les deux
parties, la cle publique stockee et l'adresse sont controlees avant usage.
Documentation officielle :
https://cryptography.io/en/latest/hazmat/primitives/asymmetric/ed25519/

## Demonstration sur le noeud local

Apres succes des tests et du comparateur Go, avec le noeud de cette branche
lance sur `http://127.0.0.1:8080` et ses wallets dans `data/` :

```powershell
cd C:\Users\ulyss\prism

& .\.venv-prism-ml\Scripts\python.exe .\tools\python\prism_ml_worker.py `
    demo --requester Alice --worker Bob --reward 1
```

Cette commande cree un job avec la vraie recompense de 1 PRISM dans le reseau
local utilise, le reclame pour Bob, calcule en Python, signe en Python et
soumet a `/complete`. Le serveur Go verifie et effectue le reglement.

Le JSON de succes attendu contient notamment :

```json
{
  "verified": true,
  "settled": true,
  "predictions": [0, 1, 0],
  "score": 12,
  "bountyReward": 1
}
```

Les vrais IDs du job, de la preuve, de la transaction et la hauteur du bloc
figurent aussi dans la sortie. Ce JSON n'est affiche qu'apres controle de la
reponse HTTP ; il s'agit de la confirmation du noeud, pas d'une verification
independante de la chaine par le client Python.

La demo ajoute le meme nonce aux deux biais. Les predictions restent
`[0, 1, 0]`, mais chaque nouvelle execution a une tache et une preuve distinctes.
Le nonce par defaut est `time.time_ns()` ; `--nonce 123` permet une execution
reproductible. Relancer avec le meme nonce peut rencontrer un job deja cree.
Changer seulement le nonce du job ne suffirait pas a changer l'ID de preuve.

## Autres commandes

Afficher l'identite publique d'un wallet :

```powershell
& .\.venv-prism-ml\Scripts\python.exe .\tools\python\prism_ml_worker.py identity --wallet Bob
```

Creer un job a partir du fichier d'exemple du premier paquet :

```powershell
& .\.venv-prism-ml\Scripts\python.exe .\tools\python\prism_ml_worker.py `
    create --requester Alice --input .\tools\python\example_quantized.json --reward 1
```

Traiter un job precis ou reprendre son propre job CLAIMED :

```powershell
& .\.venv-prism-ml\Scripts\python.exe .\tools\python\prism_ml_worker.py `
    run --worker Bob --job COLLER_ICI_LE_JOB_ID
```

Les options globales se placent avant la sous-commande :

```powershell
& .\.venv-prism-ml\Scripts\python.exe .\tools\python\prism_ml_worker.py `
    --api http://127.0.0.1:8080/api/v1 --data .\data --timeout 15 `
    run --worker Bob --job COLLER_ICI_LE_JOB_ID
```

## Contrat reproduit

- Type `ml_inference_quantized` uniquement ; entiers signes int64 avec
  controle apres chaque multiplication et addition, poids par classe,
  egalites vers la plus petite classe.
- Limites Go fournies : 4096 elements par tableau d'entrees/poids, batch et
  classes au plus 4096 ; travail au plus 1 048 576.
- Score = batch x features x classes.
- InputHash = SHA-256 du JSON compact a six champs, ordre de la structure Go.
- Task.ID = SHA-256 de `type|input_hash`.
- OutputHash = SHA-256 du JSON compact de `result_values`.
- Result scalaire = 0 pour cette preuve vectorielle.
- Proof.ID = SHA-256 de
  `task_id|worker|public_key|result|output_hash|score`.
- Signature Ed25519 sur le **texte ASCII hexadecimal** de Proof.ID.
- Adresse = `prism_` + les 20 premiers octets du SHA-256 de la cle publique.
- Job.ID = SHA-256 de `task_id|requester|reward|nonce`.

La creation passe par `{"task": ...}` deja accepte par `api_compute.go`.
Elle n'utilise pas le mode simplifie qui ne gere pas encore ce type ML.
Le client charge uniquement `data/wallets.json` pour son identite locale ;
la validation de la chaine et le reglement restent au noeud serveur.

## Comportement en cas d'erreur

Le worker valide les identifiants, le type, les dimensions et le score,
calcule et signe avant de reclamer un job OPEN. Il refuse un job CLAIMED par
un autre worker et un job deja VERIFIED. Une preuve trop volumineuse est
rejetee avant claim. La reponse claim doit correspondre au job et au wallet.

Le succes exige `verified === true` et `settled === true`, le bon job,
le bon worker, le bon Proof.ID, la recompense attendue, un ID de transaction,
une hauteur de bloc entiere et un indicateur `recovered` booleen.

Les requetes mutantes ne sont pas relancees automatiquement. Apres une erreur
reseau ou une reponse invalide, le serveur peut avoir deja traite la requete.
Conserver le Job ID affiche sur stderr, consulter l'etat du job, puis utiliser
`run` si le job est OPEN ou CLAIMED par Bob. Un job VERIFIED n'est pas soumis
de nouveau. Relancer `demo` creerait une autre tache et une autre recompense.

Le wallet prive ne figure ni dans les requetes HTTP ni dans la sortie.
Le client limite les reponses a 1 MiB, les corps POST a la limite du serveur
et refuse les redirections HTTP. Il traite un job a la fois.

## Validation effectuee et restante

Valide dans l'environnement de preparation avec Python 3.12.14 et
cryptography 46.0.0 : **49 tests reussis**, dont les 20 tests du premier paquet
et 29 nouveaux tests. Les nouveaux tests incluent la CLI complete avec des
wallets de test et un serveur HTTP simule, les signatures, les falsifications,
les confirmations incompletes, les jobs attribues a un autre worker, les
debordements avant claim, les limites HTTP et l'absence de retries.

Le serveur simule utilise le verificateur Python : il ne constitue pas une
verification independante par Go et ne produit pas de reglement blockchain.
Le comparateur Go de 16 preuves et la demo sur le vrai noeud n'ont pas ete
executes ici, car la toolchain Go et le depot complet n'y sont pas disponibles.
Ces deux etapes restent a lancer sur la machine utilisateur.

Cette version est un worker de developpement pour le protocole fourni. Elle
ne fournit pas de scheduler, de bail de claim, d'entrainement ML ou de preuve
cryptographique de calcul succincte ; le noeud Go recalcule pour verifier.
