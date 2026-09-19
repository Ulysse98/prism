# Prism : runtime et worker Python

Le runtime reproduit l'inference lineaire quantifiee de Prism en entiers
signes int64. Le worker signe les preuves Ed25519 et utilise le marketplace
HTTP du noeud Go.

- [Worker HTTP : commandes directes et format des preuves](WORKER.md)
- [Developpement v0.38 : watch, reprise et journal](WATCH.md)

## Installation depuis le depot

Python 3.10 ou plus est requis. Le runtime seul utilise la bibliotheque
standard ; les preuves signees et les tests du worker requierent cryptography.
Depuis PowerShell :

```powershell
cd C:\Users\ulyss\prism
py -m venv .venv-prism-ml
Set-Content .\.venv-prism-ml\.gitignore -Value '*' -Encoding ascii
& .\.venv-prism-ml\Scripts\python.exe -m pip install -r .\tools\python\requirements-worker.txt
& .\.venv-prism-ml\Scripts\python.exe -m unittest discover -s tools/python -v
```

Si cet environnement existe deja, reutiliser son executable Python.

## Calcul local

```powershell
& .\.venv-prism-ml\Scripts\python.exe .\tools\python\prism_ml_runtime.py .\tools\python\example_quantized.json
```

Exemple attendu :

```json
{"inputHash":"2474d76a032c9aa9016ae6e6efdf696ab6181de74ffd756916ba3e5171e31013","predictions":[0,1,0]}
```

Le fichier d'entree contient exactement `batch_size`, `features`, `classes`,
`inputs`, `weights` et `biases`. Il s'agit du payload du hash, pas du format
reseau Task ou Proof.

Les poids sont ranges par classe, les entrees par echantillon. L'accumulation
commence par le biais, controle chaque multiplication et addition en int64,
et conserve le plus petit indice de classe en cas d'egalite. Le hash porte
sur le JSON compact dans l'ordre des champs Go.

Le runtime seul ne remplace pas ValidateTask. Sa CLI a des budgets locaux
(65 536 octets et 1 000 000 multiplications). La couche prism_ml_protocol
applique les limites Go : 4096 elements et au plus 1 048 576 unites de travail.

## Comparaisons avec le vrai Go

Depuis le depot complet, avec Go disponible :

```powershell
& .\.venv-prism-ml\Scripts\python.exe .\tools\python\check_go_parity.py --repo .
& .\.venv-prism-ml\Scripts\python.exe .\tools\python\check_go_proofs.py --repo .
```

Les comparateurs creent puis suppriment un programme Go temporaire dans le
depot. Ils utilisent une cle de test publique, jamais les wallets utilisateur.
126 cas numeriques/hash et 16 cas de preuves ont reussi sous Windows pour
v0.37. Ces cas sont une verification ciblee, pas une preuve de compatibilite
pour toutes les entrees possibles. Le workflow Python worker CI les execute
avec les tests Python sous Linux et Windows.
