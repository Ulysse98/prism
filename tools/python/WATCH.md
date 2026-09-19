# v0.38 : worker automatique et reprise

Cette branche ajoute le mode watch au worker Python. Elle reste compatible
avec le noeud Go v0.37.0. La constante de version du noeud sera mise a jour
lors de la preparation de la release, apres validation de la demo reelle.

## Installation du correctif

Base du correctif : commit 95c4923ac776577554bbb362f4d6196087994ade.
Dans le depot v0.37, depuis une branche de travail propre :

```powershell
git switch -c feat/v0.38-python-watch
git apply --check "$env:USERPROFILE\Downloads\prism-v038-python-watch.patch"
git apply "$env:USERPROFILE\Downloads\prism-v038-python-watch.patch"
& .\.venv-prism-ml\Scripts\python.exe -m unittest discover -s tools/python -v
```

Si git apply --check echoue, ne pas lancer git apply. Le correctif doit etre
adapte aux changements locaux ; il ne faut pas forcer son application.

## Traiter la file

Le noeud doit etre lance dans sa propre fenetre PowerShell :

```powershell
cd C:\Users\ulyss\prism
go run .\cmd\prism api -data .\data -host 127.0.0.1 -port 8080
```

Dans une autre fenetre :

```powershell
cd C:\Users\ulyss\prism
& .\.venv-prism-ml\Scripts\python.exe .\tools\python\prism_ml_worker.py watch --worker Bob --poll-interval 2
```

Le worker traite un job a la fois. Il choisit uniquement les taches
ml_inference_quantized valides, reprend d'abord ses jobs CLAIMED, puis prend
les jobs OPEN. Les jobs d'un autre worker et les jobs dont Bob est lui-meme
le demandeur sont ignores.

Options utiles, a placer apres watch :

- `--max-jobs 3` : arret apres trois nouvelles confirmations dans cette execution.
  Les confirmations relues du journal ne sont pas recomptees.
- `--once` : un seul passage sur la file, sans attendre de nouveaux jobs.
- `--journal CHEMIN` : choisir un journal distinct. Un journal est lie a l'URL
  API exacte, au chainId, au genesisHash et a l'adresse du worker.
- `--retry-failed` : reconsiderer les jobs arretes sur une erreur permanente,
  apres en avoir corrige la cause, par exemple un solde demandeur insuffisant.

Les options globales --api, --data et --timeout se placent avant watch.

## Demonstration de trois jobs distincts

Cette commande cree trois jobs avec une prime de 1 PRISM chacun. Elle affiche
chaque identifiant des sa creation, sans execution du travail :

```powershell
& .\.venv-prism-ml\Scripts\python.exe .\tools\python\prism_ml_worker.py enqueue-demo --requester Alice --reward 1 --count 3
```

Elle utilise un nonce neuf et trois taches distinctes. Si la commande echoue,
les jobs deja crees restent dans la file et leurs IDs sont affiches ; ne pas
relancer aveuglement la creation. Le worker peut traiter ceux qui existent.

```powershell
& .\.venv-prism-ml\Scripts\python.exe .\tools\python\prism_ml_worker.py watch --worker Bob --max-jobs 3
```

Attendu si ces trois jobs sont les seuls jobs eligibles : trois evenements
JSON `confirmed`, chacun avec verified=true, settled=true, predictions [0,1,0],
score 12, bountyReward 1, proofId, settlementTxId et block. Le compteur final
est confirmed=3. Le watcher peut aussi traiter d'autres jobs eligibles deja
presents : max-jobs compte les confirmations, il ne filtre pas les trois IDs.

## Reprise apres interruption

Faire Ctrl+C dans la fenetre du worker puis relancer la meme commande watch
avec le meme journal. Le noeud peut rester en marche. Le journal par defaut est :

`data/python-worker/<adresse-du-wallet>.json`

Si l'interruption survient pendant un job, celui-ci est retrouve au prochain
passage. Si tous les jobs etaient deja confirmes, le worker attend simplement
de nouveaux jobs. Le test automatise tue aussi un vrai processus Python apres
que le serveur simule a enregistre le paiement, puis relance ce processus.

Le journal conserve les intentions avant POST et les recus apres validation,
avec remplacement atomique et synchronisation du fichier. Le verrou natif
empeche deux processus d'ecrire le meme journal et est libere par le systeme
apres un arret brutal. Le fichier .lock peut rester present : ce n'est pas en
soi le signe qu'un processus detient encore le verrou. Ne pas le supprimer
pendant une execution. Utiliser un seul watcher par wallet et API, meme si
plusieurs chemins de journaux peuvent etre fournis.

Apres une reponse perdue, watch relit le reseau et le job. Il peut alors
reclamer un job encore OPEN ou soumettre a nouveau la meme preuve pour son
propre job CLAIMED. Pour un job VERIFIED present dans son journal en attente,
il exige le meme Proof.ID avant de demander son recu a /complete.

La prevention du double paiement repose sur settleComputeJob du noeud Go,
qui retrouve la preuve et la transaction existantes. Cela couvre aussi le
cas ou le bloc a ete sauvegarde avant une panne de sauvegarde du marketplace.
Le journal ne constitue pas une preuve de reglement independante du noeud.

Les coupures, erreurs HTTP 408/429/5xx et recus incomplets sont espaces par
une attente croissante plafonnee a 60 secondes. Les autres erreurs HTTP
permanentes sont conservees comme failed. Les jobs qui debordent les limites
numeriques ou de taille sont refuses avant claim et ne bloquent pas les
autres jobs. Un journal corrompu ou d'un autre reseau provoque un arret.

Codes de sortie : 0 fin normale ; 1 erreur fatale ; 2 passage --once avec
erreur ou jobs non resolus ; 130 interruption Ctrl+C. Un --max-jobs atteint
peut retourner 0 meme si d'autres jobs sont encore non resolus ; le champ
unresolved du resume et le journal permettent de le voir.

## Limites du prototype

- Les preuves v0.37 ne lient pas l'ID du job ou du reseau dans leur signature.
  Un meme calcul pour un meme wallet produit la meme preuve. Watch refuse
  la reutilisation d'une preuve connue de son journal et, avant claim d'un
  job OPEN, verifie aussi l'historique /work. Cela ne remplace pas une future
  evolution du format de preuve ni une protection contre un serveur malveillant.
- Il n'y a pas de bail de reservation : un job CLAIMED par un worker definitivement
  absent n'est pas reattribue. La reprise utilise le meme wallet.
- Les listes /compute/jobs et /work sont encore completes, limitees a 1 MiB
  cote client ; un historique trop gros provoque une erreur explicite.
- Le journal est limite a 16 MiB. Son archivage doit conserver les recus et
  les jobs en attente ; ne pas supprimer un journal pour contourner une erreur.
- La verification initiale exige une API joignable. Les coupures apres le
  demarrage sont gerees par la boucle. Le noeud Go recalcule les preuves.

## Verification de cette livraison

76 tests Python ont reussi dans l'environnement Linux de preparation
(Python 3.12.14, cryptography 46.0.0), dont les 49 tests de v0.37. La branche
Windows du verrou et la CI distante ne sont pas declarees validees ici.

Les tests couvrent la file de trois jobs, les pertes de reponse claim/complete,
la reprise d'un job deja regle, l'arret force d'un processus, l'exclusion
mutuelle, l'echec d'ecriture, les identites de reseau, la concurrence entre
workers, les erreurs HTTP et les recus incomplets. Le serveur de ces tests
est un simulateur Python dont la reprise reproduit le contrat Go inspecte.

Le workflow Python worker CI lance ces tests et les comparateurs Go sous
Linux et Windows. Le test Ctrl+C POSIX est ignore sous Windows ; le test
d'arret force et de liberation du verrou y reste actif.

L'environnement de preparation n'a pas Go : la demo de trois jobs et la
reprise contre le vrai noeud restent a executer sur la machine utilisateur.
Le noeud a deja son test Go TestSettleComputeJobRetryDoesNotDoublePay, execute
par la CI Go existante.
