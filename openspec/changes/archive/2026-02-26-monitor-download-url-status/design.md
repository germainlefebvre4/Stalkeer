## Context

`DownloadInfo` est la table centrale de tracking des téléchargements. Elle contient statut, retry_count, bytes, timestamps et error_message. Deux lacunes :

1. **L'URL n'est pas stockée** : elle vit dans `ProcessedLine.LineURL`. Toute requête "quel est l'état de cette URL ?" nécessite un JOIN.
2. **`retry_count` n'est jamais incrémenté pendant les retries** : `retry.Do()` enchaîne les tentatives silencieusement (sans mise à jour DB). La valeur reste à 0 même après 3 tentatives.

Le système utilise `retry.Do()` avec une fonction de callback, et `StateManager.UpdateState(Retrying)` existe mais n'est jamais appelé.

## Goals / Non-Goals

**Goals:**
- Stocker l'URL dans `DownloadInfo` pour des requêtes SQL directes
- Incrémenter `retry_count` en base entre chaque tentative échouée
- Ne rien casser au comportement existant

**Non-Goals:**
- Historique par tentative (pas de table `download_attempts`)
- Endpoints API dédiés aux downloads
- Métriques temps-réel ou dashboard
- Tracking du code HTTP par tentative

## Decisions

### D1 — Ajouter `url` dans `DownloadInfo` (pas un join)

**Choix** : ajouter `URL string` à `DownloadInfo`, populé depuis `opts.URL` lors de `getOrCreateDownloadInfo`.

**Alternatif écarté** : requêter via JOIN `processed_lines.line_url`. Trop lourd pour des requêtes ad-hoc SQL et rompt l'autonomie de `DownloadInfo`.

**Rationale** : `DownloadInfo` doit être auto-suffisante pour le monitoring. Le coût (duplication de l'URL) est négligeable.

### D2 — Incrémenter `retry_count` via callback dans `retry.Do()`

**Choix** : modifier `retry.Do()` pour accepter un callback optionnel `OnRetry func(attempt int, err error)`, appelé avant chaque sleep de retry. Le downloader passe un callback qui appelle `StateManager.UpdateState(Retrying)`.

**Alternatif écarté** : gérer les retries manuellement dans le downloader (boucle explicite). Casse la séparation des responsabilités et duplique la logique de backoff.

**Alternatif écarté** : ticker/goroutine séparé. Sur-ingénierie pour un besoin synchrone.

**Rationale** : minimal, non-breaking (callback optionnel), conserve toute la logique de backoff dans `retry`.

### D3 — Migration DB additive

**Choix** : colonne `url` nullable (`*string` ou `string` avec valeur vide par défaut) pour compatibilité avec les records existants.

**Rationale** : les `DownloadInfo` créées avant la migration auront URL vide — acceptable pour un outil de monitoring. Pas de migration des données historiques nécessaire.

## Risks / Trade-offs

- **Duplication URL** : l'URL est dans `ProcessedLine.LineURL` ET `DownloadInfo.URL`. Si une URL change (rare), les deux peuvent diverger. → Acceptable : l'URL de download est immutable une fois le téléchargement lancé.
- **`retry_count` toujours à 0 pour anciens records** : les downloads déjà en base n'auront pas de retry_count rétroactif. → Acceptable : monitoring pour les nouveaux downloads.
- **`OnRetry` callback** : si le callback plante (erreur DB), le retry continue quand même. → Choix délibéré : les erreurs de tracking ne doivent pas bloquer le download.

## Migration Plan

1. Ajouter `URL string` à `DownloadInfo` (GORM auto-migrate au démarrage)
2. Modifier `retry.Config` pour ajouter `OnRetry func(attempt int, err error)` (optionnel, nil-safe)
3. Modifier `retry.Do()` pour appeler `OnRetry` avant chaque sleep
4. Modifier `getOrCreateDownloadInfo` pour peupler `URL` depuis `opts.URL`
5. Dans `Download()`, passer un `OnRetry` callback qui incrémente `retry_count` via `StateManager`

Rollback : supprimer la colonne `url` (nullable → pas d'impact) + revert code.
