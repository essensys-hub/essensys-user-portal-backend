# Fusion des commandes du portail

> Feature `portal-action-merge-2026-10-007` — essensys-hub/essensys-user-portal-backend#33 (statut : proposée)

Quand plusieurs commandes du portail attendent la même armoire, le hub les livre ensemble. La livraison doit avoir le même effet que les commandes appliquées une à une :

- les lampes et volets d'une même zone se **cumulent** (OU bit à bit sur les index 605–622) ;
- pour une même lampe ou un même volet, la **dernière demande gagne** (allumer puis éteindre → éteindre) ;
- le bloc 605–622 est toujours complet, 590 = `"1"`, 30 paramètres au plus par action ;
- les autres commandes (scénarios, chauffage…) sont livrées une par une, dans l'ordre.

Pour l'utilisateur : « Tout allumer » allume toutes les lampes, même si l'armoire reçoit les 21 commandes en une seule fois.

Conception : `essensys-memory/openspec/changes/portal-action-merge-2026-10-007/`.
