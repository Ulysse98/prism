# Prism v0.37 : moteur Python quantifie

Premiere etape du worker Python, basee sur `ml_inference_quantized.go` fourni
depuis `feat/v0.37-pouw-runtime`, commit `b83445e`.

Ce paquet fournit le calcul local, le hash des entrees et une comparaison
avec le vrai package Go `prism/internal/usefulwork`. Python 3.10 ou plus,
bibliotheque standard uniquement. Aucune installation pip.

## Utilisation sous PowerShell

Enregistrer `prism-python-v037.zip` dans Downloads puis, depuis le depot :

```powershell
cd C:\Users\ulyss\prism
Expand-Archive -LiteralPath "$env:USERPROFILE\Downloads\prism-python-v037.zip" -DestinationPath .
py -m unittest discover -s tools/python -v
py .\tools\python\prism_ml_runtime.py .\tools\python\example_quantized.json
py .\tools\python\check_go_parity.py --repo .
```

L'archive contient seulement `tools/python/`. L'extraction standard sans
`-Force` refuse d'ecraser les fichiers existants de meme nom.

Exemple attendu :

```json
{"inputHash":"2474d76a032c9aa9016ae6e6efdf696ab6181de74ffd756916ba3e5171e31013","predictions":[0,1,0]}
```

Il s'agit d'un nouvel exemple de calcul, pas des entrees du test E2E precedent.

## Regles numeriques reproduites

- Entrees et poids signes int64 ; biais par classe.
- Entrees contigues par echantillon ; poids contigus par classe.
- Accumulation dans l'ordre des features, en partant du biais.
- Verification int64 apres chaque multiplication ET chaque addition.
- Egalite de scores : plus petit indice de classe.
- SHA-256 du JSON compact des six champs dans l'ordre exact de la structure Go.

Le JSON utilise ici est le **payload du hash**, avec `batch_size`, `features`,
`classes`, `inputs`, `weights` et `biases`. Ce n'est pas le JSON reseau `Task`
ni un `Proof`. Le runtime refuse les champs inconnus et les cles dupliquees.

Python utilise des entiers arbitraires : les controles explicites sont
indispensables pour reproduire les rejets Go. Une somme finale representable
doit quand meme etre rejetee si une etape intermediaire deborde.

Les limites de taille et de travail du protocole Go n'ont pas ete fournies.
Le module reproduit le calcul et le hash sur les petites taches bien formees ;
il ne remplace pas `ValidateTask`. La CLI applique ses propres budgets locaux
(65 536 octets et 1 000 000 multiplications), qui ne sont pas des constantes
de consensus. Le serveur Go reste l'autorite pour la validation des taches.

## Verification

20 tests Python couvrent predictions, disposition des poids, biais,
egalites, limites int64, debordements intermediaires et entrees invalides.
Ils ont ete executes avec succes sous Python 3.12.14 dans l'environnement
de preparation. Python 3.14.6 est la version detectee sur la machine utilisateur.

`check_go_parity.py` prepare 126 cas : 26 cas cibles et 100 petits cas
pseudo-aleatoires reproductibles. Il compare les predictions et `InputHash`,
ainsi que les categories d'erreurs pour les cas rejetes. Il importe les vraies
fonctions `NewMLInferenceQuantizedTask` et `ComputeMLInferenceQuantized`.
Il cree un programme temporaire dans le depot, utilise `go run -mod=readonly`,
puis supprime ce programme. La premiere compilation peut prendre du temps.

La comparaison Go n'a pas ete executee dans l'environnement de preparation :
Go et le depot complet n'y sont pas disponibles. Elle doit reussir chez
l'utilisateur avant de conclure a la parite entre langages sur ces cas.
Ce test cible ne constitue pas une preuve de compatibilite pour tous les inputs.

## Raccordement HTTP suivant

Dans le fichier `api_compute.go` fourni, le mode simplifie `type` + champs
ne gere pas encore `ml_inference_quantized`. Le mode `task` imbrique appelle
`ValidateTask` et permet de soumettre une tache complete valide ; les etapes
suivantes du marketplace et du reglement restent a inspecter.

Il manque notamment les definitions JSON de Task/Proof, les hashes/identifiants
de preuve et les exigences du worker/reglement. Le paquet ne reclame pas de
job et ne soumet aucune preuve. Il ne signe rien et ne declenche aucune recompense.

Pour preparer la suite, fournir :

```powershell
Get-Content .\internal\usefulwork\usefulwork.go
Get-Content .\cmd\prism\compute_worker.go
```

L'inspection de ces fichiers determinera les autres dependances a lire avant
le raccordement `claim -> inference -> complete` et le test E2E de reglement.
