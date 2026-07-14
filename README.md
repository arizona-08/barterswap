# BarterSwap — API d'échange de compétences

API REST écrite en Go avec `net/http`, `database/sql` et PostgreSQL. La partie actuellement implémentée couvre la gestion des utilisateurs et de leurs compétences.

## Lancement avec Docker

```bash
docker compose up --build -d
docker compose exec go-dev go run .
```

L'API écoute sur `http://localhost:8080`. Le schéma PostgreSQL est créé automatiquement au démarrage.

## Endpoints utilisateurs

| Méthode | Route | Description | Authentification |
|---|---|---|---|
| `POST` | `/api/users` | Créer un utilisateur avec 10 crédits | Non |
| `GET` | `/api/users/{id}` | Consulter un profil public | Non |
| `PUT` | `/api/users/{id}` | Remplacer les informations du profil | `X-User-ID` |
| `GET` | `/api/users/{id}/skills` | Consulter les compétences | Non |
| `PUT` | `/api/users/{id}/skills` | Remplacer toutes les compétences | `X-User-ID` |

## Exemples

```bash
curl -X POST http://localhost:8080/api/users \
  -H 'Content-Type: application/json' \
  -d '{"pseudo":"Alice","bio":"Passionnée de jardinage","ville":"Paris"}'
```

```bash
curl http://localhost:8080/api/users/1
```

```bash
curl -X PUT http://localhost:8080/api/users/1 \
  -H 'Content-Type: application/json' \
  -H 'X-User-ID: 1' \
  -d '{"pseudo":"Alice","bio":"Jardinière amateure","ville":"Lyon"}'
```

```bash
curl -X PUT http://localhost:8080/api/users/1/skills \
  -H 'Content-Type: application/json' \
  -H 'X-User-ID: 1' \
  -d '{"skills":[{"nom":"Jardinage","niveau":"expert"},{"nom":"Cuisine","niveau":"intermédiaire"}]}'
```

Les niveaux autorisés sont `débutant`, `intermédiaire` et `expert`. Chaque `PUT` sur les compétences remplace la liste précédente.

## Tests

```bash
docker compose exec go-dev go test -v -cover ./...
```
