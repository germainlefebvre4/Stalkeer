## 1. Title normalization in extractTitleAndYear

- [x] 1.1 Ajouter le strip des suffixes qualité (`SD`, `HD`, `FHD`, `UHD`, `4K`, `MULTI`, `VOSTFR`, `VF`) en fin de titre dans `extractTitleAndYear` (regex `\s+(SD|FHD|HD|UHD|4K|MULTI|VOSTFR|VF)(\s+.*)?$`, case-insensitive)
- [x] 1.2 Ajouter la détection du format `Titre - YYYY` dans `extractTitleAndYear` (regex `\s*-\s*((?:19|20)\d{2})$` appliquée après strip qualité)
- [x] 1.3 Vérifier que le cas `Spider-Man : No Way Home` (tiret dans le titre sans année) n'est pas altéré

## 2. Atomic upsert pour Movie

- [x] 2.1 Remplacer le bloc `Where(...).First() + ErrRecordNotFound + Create()` dans `enrichMovie` par `db.Where(...).Attrs(movie).FirstOrCreate(&movie)`
- [x] 2.2 Conserver la logique de mise à jour du TVDB ID si manquant (branche `else if`)

## 3. Atomic upsert pour TVShow

- [x] 3.1 Remplacer le bloc `Where(...).First() + ErrRecordNotFound + Create()` dans `enrichTVShow` par `db.Where(...).Attrs(tvshow).FirstOrCreate(&tvshow)`
- [x] 3.2 Conserver la logique de mise à jour du TVDB ID si manquant (branche `else if`)

## 4. Tests

- [x] 4.1 Ajouter / mettre à jour les tests unitaires de `extractTitleAndYear` pour couvrir : suffix SD, suffix FHD MULTI, format `- YYYY`, titre sans modification, tiret sans année
- [x] 4.2 Vérifier que les tests existants du package `processor` passent après les modifications
