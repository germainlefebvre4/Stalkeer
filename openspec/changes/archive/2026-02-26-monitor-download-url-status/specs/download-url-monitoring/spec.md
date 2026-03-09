## ADDED Requirements

### Requirement: URL stockée dans DownloadInfo
Le système SHALL stocker l'URL de téléchargement directement dans `DownloadInfo` lors de la création du record, afin de permettre des requêtes SQL sans JOIN sur `processed_lines`.

#### Scenario: Création d'un DownloadInfo avec URL
- **WHEN** un téléchargement est initié avec une URL non vide
- **THEN** le record `DownloadInfo` créé en base contient cette URL dans le champ `url`

#### Scenario: Record DownloadInfo existant
- **WHEN** un `DownloadInfo` existe déjà pour une `ProcessedLine` donnée
- **THEN** l'URL n'est pas modifiée (idempotent)

### Requirement: retry_count incrémenté entre chaque tentative
Le système SHALL incrémenter `retry_count` dans `DownloadInfo` après chaque tentative échouée et avant la prochaine tentative, pas seulement à l'échec final.

#### Scenario: Première tentative échoue, deuxième réussit
- **WHEN** la première tentative de téléchargement échoue avec une erreur retryable
- **THEN** `retry_count` est incrémenté à 1 en base avant la deuxième tentative

#### Scenario: Toutes les tentatives échouent
- **WHEN** toutes les tentatives (MaxAttempts) échouent
- **THEN** `retry_count` reflète le nombre total de retries effectués (MaxAttempts - 1)

#### Scenario: Erreur de tracking n'interrompt pas le download
- **WHEN** l'incrémentation de `retry_count` échoue (erreur DB)
- **THEN** le retry du téléchargement continue normalement

### Requirement: Queryabilité SQL de l'état par URL
Le système SHALL permettre de requêter l'état, le nombre de tentatives et le message d'erreur d'un téléchargement directement sur la table `download_info` par URL, sans JOIN.

#### Scenario: Requête état par URL
- **WHEN** une requête SQL `SELECT * FROM download_info WHERE url = '<url>'` est exécutée
- **THEN** elle retourne le statut, retry_count, error_message, started_at, completed_at du téléchargement

#### Scenario: Requête agrégée succès/échec
- **WHEN** une requête SQL `SELECT status, COUNT(*) FROM download_info GROUP BY status` est exécutée
- **THEN** elle retourne les comptes corrects par statut sans JOIN nécessaire
