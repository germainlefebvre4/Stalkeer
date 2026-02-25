## Context

Le workflow `process` enrichit chaque entrée M3U classifiée comme film ou série via l'API TMDB. Les titres M3U sont fournis par des opérateurs IPTV avec des suffixes propriétaires (`SD`, `HD`, `FHD`, `MULTI`…) et parfois une année au format `Titre - YYYY`. Ces artefacts ne sont pas reconnus par TMDB et provoquent un volume élevé de warnings `no results found`. En parallèle, plusieurs variants d'un même film dans un même lot (HD + FHD + SD) déclenchent un pattern check-then-insert non atomique qui génère des erreurs `duplicate key`.

## Goals / Non-Goals

**Goals:**
- Réduire les faux `TMDBNotFound` causés par des suffixes qualité non nettoyés
- Reconnaître le format `Titre - YYYY` pour l'extraction d'année
- Éliminer les erreurs `duplicate key` sur `movies` et `tv_shows` via upsert atomique

**Non-Goals:**
- Implémenter une stratégie multi-pass TMDB (fallback sans année, fallback en-US) — hors scope
- Modifier la logique de nettoyage des séries (`cleanTVShowTitle`) — elle est déjà correcte
- Changer le schéma de base de données

## Decisions

### D1 — Factoriser le nettoyage titre dans `extractTitleAndYear`

**Décision** : Modifier `extractTitleAndYear` pour (1) détecter et strip le format `- YYYY` avant le nettoyage des suffixes, et (2) appliquer un strip des tokens qualité en fin de titre.

**Logique d'ordre des opérations** :
```
Input titre brut
    │
    ▼
Strip suffixes qualité en queue (SD|HD|FHD|UHD|4K|MULTI|VOSTFR|VF)
    │
    ▼
Extraire année — tenter "(YYYY)" puis "- YYYY"
    │
    ▼
(titre propre, *année)
```

**Pourquoi pas une fonction séparée ?** La fonction `extractTitleAndYear` est déjà l'unique point d'entrée pour les films. Centraliser le nettoyage ici évite la dispersion et reste cohérent avec `cleanTVShowTitle` pour les séries.

**Alternative rejetée** : Appliquer les mêmes patterns que `cleanTVShowTitle` directement. Rejeté car `cleanTVShowTitle` est conçu pour les séries (patterns S01E01, etc.) — dupliquer ces patterns dans le code film serait trompeur.

### D2 — Upsert atomique avec `FirstOrCreate`

**Décision** : Remplacer le pattern `Where(...).First() + Create()` par `FirstOrCreate()` de GORM dans `enrichMovie` et `enrichTVShow`.

```
Avant :
  db.Where("tmdb_id = ? AND tmdb_year = ?", ...).First(&movie)  ← racy
  if ErrRecordNotFound → db.Create(&movie)                       ← duplicate possible

Après :
  db.Where(...).Attrs(movie).FirstOrCreate(&movie)               ← atomique
```

**Pourquoi `FirstOrCreate` plutôt que `ON CONFLICT DO NOTHING`** ? `FirstOrCreate` est idiomatique GORM, portable (Postgres + SQLite), et retourne l'entité existante ou la nouvelle — utile pour récupérer l'ID. Un raw `ON CONFLICT` nécessiterait du SQL brut et casse la portabilité.

**Limitation connue** : `FirstOrCreate` n'est pas strictement atomique sous haute concurrence (race window entre SELECT et INSERT côté GORM). Pour ce workflow single-process, ce niveau est suffisant. Si un parallélisme est introduit plus tard, basculer vers `INSERT ... ON CONFLICT DO NOTHING`.

## Risks / Trade-offs

- **[Risk] Regex trop agressive sur les titres** → Un titre légitime contenant "SD" ou "HD" en queue pourrait être tronqué (ex: un film dont l'acronyme finit par ces lettres). Mitigation : la regex cible uniquement des tokens séparés par un espace, pas des sous-chaînes. Probabilité faible.
- **[Risk] Format `- YYYY` ambigu** → Un titre comme `"Mission : Impossible - Fallout"` ne doit pas être altéré. Mitigation : la regex doit cibler explicitement `\s*-\s*(19|20)\d{2}$` (fin de chaîne, 4 chiffres avec préfixe siècle valide).
- **[Trade-off] `FirstOrCreate` vs upsert SQL brut** → `FirstOrCreate` effectue un SELECT puis éventuellement un INSERT, soit 2 requêtes vs 1 pour un upsert natif. Acceptable pour ce cas d'usage séquentiel.

## Migration Plan

Aucune migration de données requise. Les changements sont purement applicatifs :
1. Modifier `extractTitleAndYear` dans `processor.go`
2. Modifier `enrichMovie` et `enrichTVShow` dans `processor.go`
3. Mettre à jour les tests unitaires dans `processor_test.go` (si existants) ou `tmdb_test.go`

Rollback : revert du commit — aucun état persistant à défaire.
