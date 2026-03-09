## 1. Modèle & Migration DB

- [x] 1.1 Ajouter le champ `URL string` à `DownloadInfo` dans `internal/models/processing_log.go`

## 2. Retry avec callback

- [x] 2.1 Ajouter le champ `OnRetry func(attempt int, err error)` (optionnel) à `retry.Config` dans `internal/retry/retry.go`
- [x] 2.2 Appeler `OnRetry` dans `retry.Do()` après chaque échec retryable, avant le sleep

## 3. Downloader — URL et retry tracking

- [x] 3.1 Peupler `URL` depuis `opts.URL` dans `getOrCreateDownloadInfo()` (`internal/downloader/downloader.go`)
- [x] 3.2 Passer un callback `OnRetry` à `retry.Do()` qui appelle `StateManager.UpdateState(Retrying)` pour incrémenter `retry_count`

## 4. Tests

- [x] 4.1 Vérifier que `DownloadInfo.URL` est peuplé lors d'un download (test existant ou nouveau)
- [x] 4.2 Vérifier que `retry_count` est incrémenté entre les tentatives
