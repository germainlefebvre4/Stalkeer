## Why

Le workflow `process` génère un volume élevé de warnings `failed to enrich movie with TMDB` parce que les titres M3U contiennent des suffixes de qualité (`SD`, `HD`, `FHD`, `MULTI`…) qui ne sont pas strippés avant la recherche, et parce que le format d'année `- YYYY` n'est pas reconnu. En parallèle, des entrées identiques traitées en lot provoquent des erreurs `duplicate key` en base à cause d'un pattern check-then-insert non atomique.

## What Changes

- **Title cleaning** : `extractTitleAndYear` strip désormais les suffixes de qualité (`SD`, `HD`, `FHD`, `UHD`, `MULTI`, `VOSTFR`, `VF`) et reconnaît le format `Titre - YYYY` en plus de `Titre (YYYY)`.
- **DB upsert** : les insertions de `Movie` et `TVShow` dans `enrichMovie` / `enrichTVShow` utilisent un upsert atomique (`FirstOrCreate` ou `ON CONFLICT DO NOTHING`) à la place du pattern check-then-insert.

## Capabilities

### New Capabilities
- `m3u-title-normalization`: Nettoyage et normalisation des titres M3U avant recherche TMDB (strip suffixes qualité, extraction d'année multi-format).

### Modified Capabilities
- `tmdb-rate-limiter`: Aucun changement de requirements — pas de delta spec requis.

## Impact

- `internal/processor/processor.go` : fonctions `extractTitleAndYear`, `enrichMovie`, `enrichTVShow`
- Aucune migration DB, aucune modification d'API publique, aucune dépendance externe nouvelle
