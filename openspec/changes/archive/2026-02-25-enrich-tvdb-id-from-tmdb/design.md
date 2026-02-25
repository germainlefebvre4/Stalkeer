## Context

Les enregistrements `Movie` et `TVShow` stockent un `tmdb_id` et un `tvdb_id` optionnel. Lors du processing M3U, `GetMovieExternalIDs` / `GetTVShowExternalIDs` est appelé mais peut échouer (erreur API, rate limit). Le résultat : `tvdb_id = NULL` en base. Les runs suivants ignorent ces lignes comme doublons — le NULL persiste indéfiniment.

Le matcher Sonarr (`MatchTVShowByTVDB`) s'appuie sur `tvdb_id` comme clé primaire de matching. Un `tvdb_id` manquant force un fallback fuzzy peu fiable.

Infrastructure disponible : `tmdb.Client` avec rate limiting, circuit breaker et cache en mémoire — il suffit de le réutiliser.

## Goals / Non-Goals

**Goals:**
- Peupler rétroactivement `tvdb_id` sur tous les Movie/TVShow avec `tmdb_id != 0` et `tvdb_id IS NULL`
- Exposer cette logique comme commande CLI `enrich-tvdb` pour une exécution à la demande
- Respecter le rate limiting TMDB existant (réutilisation du client configuré)

**Non-Goals:**
- Modifier le flux `process` M3U (il tente déjà le backfill sur chaque run)
- Intégrer le backfill en temps réel dans le matcher Sonarr/Radarr (séparation des responsabilités)
- Mettre à jour d'autres champs TMDB (titre, genres, durée) — seul `tvdb_id` est ciblé

## Decisions

### D1 — Commande CLI dédiée plutôt qu'un flag sur `process`

**Décision** : Nouvelle commande `stalkeer enrich-tvdb` plutôt qu'un flag `--backfill-tvdb` sur `process`.

**Rationale** : `process` est centré sur le parsing M3U. Une commande dédiée peut être lancée indépendamment (ex: après un run initial, sur cron), est plus prévisible, et permet un `--dry-run` et un `--limit` indépendants.

**Alternative rejetée** : Intégrer dans `process --force`. Rejeté car `--force` re-traite toutes les lignes, beaucoup trop coûteux juste pour corriger des TVDB IDs manquants.

### D2 — Logique dans le package `processor` (méthode sur `Processor`) ou package dédié

**Décision** : Nouvelle fonction package-level `EnrichMissingTVDBIDs(db, tmdbClient, opts)` dans `internal/processor/` — pas de nouveau package.

**Rationale** : La logique est courte (fetch + update en batch), cohérente avec le reste du processing TMDB dans ce package, et évite la création d'un package pour quelques dizaines de lignes.

### D3 — Traitement en batch avec pagination

**Décision** : Traiter les enregistrements par batch de N (configurable, défaut 100) avec `LIMIT/OFFSET` pour éviter de charger toute la table en mémoire.

**Rationale** : La table peut contenir des centaines de milliers de lignes. Traitement séquentiel respecte le rate limiter TMDB naturellement.

## Risks / Trade-offs

- **[Risk] `GetMovieExternalIDs` échoue à nouveau** → Le run suivant retente. Mitigation : log WARN + continuer, pas de blocage.
- **[Risk] Un film a un TMDB ID valide mais pas de TVDB ID côté TMDB** → `tvdb_id` reste NULL. Accepté — ce n'est pas une erreur, certains films n'ont pas d'entrée TVDB.
- **[Risk] Rate limiting TMDB** → Mitigation : le client existant gère déjà le throttling et le retry. La commande s'appuie dessus.
- **[Trade-off] Pagination OFFSET** → Peut être lent sur très grandes tables. Acceptable pour une commande de maintenance lancée manuellement.

## Migration Plan

Pas de migration de schéma. La commande peut être lancée une fois après déploiement pour backfiller les données existantes :

```bash
stalkeer enrich-tvdb --dry-run      # prévisualisation
stalkeer enrich-tvdb --limit 1000   # batch progressif
stalkeer enrich-tvdb                # traitement complet
```

Rollback : aucun — la commande ne fait que SET tvdb_id = <valeur>, réversible manuellement si nécessaire.
