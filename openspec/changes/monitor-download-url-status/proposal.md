## Why

Les downloads sont actuellement traçables via `DownloadInfo`, mais l'URL source n'y est pas stockée et `retry_count` n'est jamais incrémenté pendant les tentatives. Il est impossible de faire une requête SQL simple pour connaître l'état d'une URL, le nombre de tentatives réelles, ou les raisons d'échec.

## What Changes

- Ajouter un champ `url` à `DownloadInfo` (migration DB), populé à la création du record
- Incrémenter `retry_count` en base entre chaque tentative échouée (pas seulement sur l'échec final)
- Permettre des requêtes SQL directes sur `download_info` pour monitorer état, tentatives, succès/échecs par URL

## Capabilities

### New Capabilities

- `download-url-monitoring`: Tracking de l'état de download par URL — url, statut, nombre de tentatives, message d'erreur — queryable directement en SQL sur la table `download_info`

### Modified Capabilities

<!-- Aucune spec existante n'est impactée — pas de changement de comportement observable externe -->

## Impact

- **DB** : migration pour ajouter colonne `url` à `download_info`
- **Code** : `internal/models/processing_log.go` — champ `URL` sur `DownloadInfo`
- **Code** : `internal/downloader/downloader.go` — peupler `URL` à la création, incrémenter `retry_count` entre tentatives
- **Code** : `internal/downloader/state_manager.go` — méthode ou callback pour incrémenter `retry_count`
- **Aucun** : pas d'API, pas de nouveau endpoint, pas de nouvelle table
