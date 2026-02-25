## 1. Logique de backfill dans internal/processor

- [x] 1.1 Créer la fonction `EnrichMissingTVDBIDs(db *gorm.DB, client *tmdb.Client, opts EnrichTVDBOptions) (*EnrichTVDBStats, error)` dans `internal/processor/` (nouveau fichier `enrich_tvdb.go`)
- [x] 1.2 Implémenter le backfill Movie : requête `WHERE tvdb_id IS NULL AND tmdb_id != 0`, itération par batch, appel `GetMovieExternalIDs`, update si tvdb_id non-nil
- [x] 1.3 Implémenter le backfill TVShow : même logique avec déduplication par `tmdb_id` (une seule requête API par `tmdb_id` unique, mise à jour de toutes les lignes correspondantes)
- [x] 1.4 Définir les types `EnrichTVDBOptions` (DryRun bool, Limit int, Verbose bool) et `EnrichTVDBStats` (Processed, Updated, Skipped, Errors int)

## 2. Commande CLI `enrich-tvdb`

- [x] 2.1 Ajouter `var enrichTVDBCmd = &cobra.Command{...}` dans `cmd/main.go`
- [x] 2.2 Câbler les flags `--dry-run`, `--limit`, `--verbose`
- [x] 2.3 Instancier le `tmdb.Client` depuis la config (réutiliser le même pattern que `processCmd`)
- [x] 2.4 Appeler `processor.EnrichMissingTVDBIDs(...)` et afficher le résumé final (total, updated, skipped, errors)
- [x] 2.5 Enregistrer la commande avec `rootCmd.AddCommand(enrichTVDBCmd)`

## 3. Tests

- [x] 3.1 Ajouter des tests unitaires pour `EnrichMissingTVDBIDs` couvrant : movie mis à jour, movie sans TVDB sur TMDB (skipped), erreur API (continue), déduplication TVShow par tmdb_id
- [x] 3.2 Vérifier que `go build ./internal/... ./cmd/...` passe sans erreur
