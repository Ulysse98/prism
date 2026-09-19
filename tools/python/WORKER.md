# Worker Python HTTP signe

Le worker prend en charge `ml_inference_quantized`, avec le noeud Go v0.37.0.
Voir [README.md](README.md) pour l'environnement Python et les comparateurs,
et [WATCH.md](WATCH.md) pour le mode automatique ajoute dans la branche v0.38.

## Commandes directes

Avec le noeud local lance sur `http://127.0.0.1:8080` et ses wallets dans data :

```powershell
cd C:\Users\ulyss\prism
& .\.venv-prism-ml\Scripts\python.exe .\tools\python\prism_ml_worker.py identity --wallet Bob
& .\.venv-prism-ml\Scripts\python.exe .\tools\python\prism_ml_worker.py demo --requester Alice --worker Bob --reward 1
```

`demo` cree un job avec une prime de 1 PRISM, le reclame pour Bob, calcule et
signe en Python, puis soumet la preuve au noeud pour verification et reglement.
Une demo de v0.37 a retourne verified=true, settled=true, predictions [0,1,0],
score 12 et bountyReward 1 au bloc 49 sur prism-d8c1f3e740b48957.

Pour un fichier de parametres ou un job existant :

```powershell
& .\.venv-prism-ml\Scripts\python.exe .\tools\python\prism_ml_worker.py create --requester Alice --input .\tools\python\example_quantized.json --reward 1
& .\.venv-prism-ml\Scripts\python.exe .\tools\python\prism_ml_worker.py run --worker Bob --job COLLER_LE_JOB_ID
```

Les options globales --api, --data et --timeout se placent avant la
sous-commande. `run` accepte un job OPEN ou deja CLAIMED par ce wallet et
refuse un job VERIFIED. Il ne relance pas automatiquement les POST.
Apres une erreur reseau, conserver le Job ID affiche et lire l'etat du job :
le noeud peut avoir traite la requete. Relancer demo creerait un autre job.

## Format reproduit

- Entiers signes int64 avec controle de chaque operation et egalite vers
  la plus petite classe ; poids par classe et biais par classe.
- Score = batch x features x classes.
- InputHash = SHA-256 du JSON compact des six champs dans l'ordre Go.
- Task.ID = SHA-256 de `type|input_hash`.
- OutputHash = SHA-256 du JSON compact des predictions.
- Result scalaire = 0 pour une preuve vectorielle.
- Proof.ID = SHA-256 de
  `task_id|worker|public_key|result|output_hash|score`.
- Signature Ed25519 sur le texte ASCII hexadecimal de Proof.ID.
- Adresse = `prism_` + les 20 premiers octets du SHA-256 de la cle publique.
- Job.ID = SHA-256 de `task_id|requester|reward|nonce`.

La cle Go est stockee sous la forme seed || public key, sur 64 octets.
Le worker utilise la seed de 32 octets avec cryptography et verifie la cle
publique, le suffixe prive et l'adresse. Documentation :
https://cryptography.io/en/latest/hazmat/primitives/asymmetric/ed25519/

Le wallet prive reste local. Les requetes transmettent la preuve signee et
la cle publique. Les reponses HTTP sont limitees a 1 MiB et les redirections
sont refusees.

Le succes exige les booleens verified et settled, le bon job, le bon worker,
le bon Proof.ID, la prime attendue, un identifiant de transaction, une hauteur
entiere et un indicateur recovered booleen. Il s'agit du recu du noeud ; le
client Python ne verifie pas independamment la chaine. Le noeud recalcule
le travail. Il n'y a pas de preuve de calcul succincte ou zero knowledge.

## Validation

Pour v0.37 : 49 tests Python, 126 comparaisons numeriques/hash, 16 cas de
preuves et la demo HTTP avec reglement ont reussi sur la machine Windows.
Les cas supplementaires de reprise v0.38 et leurs limites sont decrits dans
[WATCH.md](WATCH.md).
