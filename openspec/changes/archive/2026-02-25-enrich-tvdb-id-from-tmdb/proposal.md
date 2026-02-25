## Why

Les enregistrements `Movie` et `TVShow` en base peuvent avoir `tvdb_id = NULL` si l'appel `GetMovieExternalIDs` / `GetTVShowExternalIDs` a échoué lors du processing, ou si la ligne M3U est détectée comme doublon aux runs suivants (bloquant tout re-enrichissement). Sans `tvdb_id`, le matcher Sonarr — qui utilise le TVDB ID comme clé primaire — tombe en fallback fuzzy et rate des correspondances.

## What Changes

- **Nouvelle commande `enrich-tvdb`** : parcourt tous les `Movie` et `TVShow` en DB dont `tvdb_id` est NULL et `tmdb_id` est renseigné, appelle TMDB (`GetMovieExternalIDs` / `GetTVShowExternalIDs`), et met à jour les enregistrements.
- **Intégration au rate limiter TMDB existant** : la commande réutilise le `tmdb.Client` configuré (rate limiting, circuit breaker, cache) sans nouvelle dépendance.

## Capabilities

### New Capabilities
- `tvdb-id-backfill`: Commande de backfill pour peupler le champ `tvdb_id` manquant sur les enregistrements Movie/TVShow existants à partir de l'API TMDB External IDs.

### Modified Capabilities
_(aucun changement de requirements sur les capabilities existantes)_

## Impact

- `cmd/main.go` : nouveau sous-commande `enrich-tvdb` (cobra)
- `internal/processor/processor.go` ou nouveau package `internal/enricher/` : logique de backfill
- Aucune migration DB (colonnes `tvdb_id` déjà présentes et nullables)
- Réutilise `tmdb.Client` configuré — pas de nouvelle clé API
