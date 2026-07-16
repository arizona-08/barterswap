# BarterSwap — API d'échange de compétences

API REST écrite en Go avec `net/http`, `database/sql` et PostgreSQL. Le projet
couvre actuellement les utilisateurs, les compétences, les annonces de services,
les échanges, les crédits, les avis et les statistiques.

## Lancement avec Docker

```bash
cp .env.example .env
docker compose up --build -d
docker compose exec go-dev go run .
```

L'API écoute sur `http://localhost:8080`. Le schéma PostgreSQL est créé
automatiquement au démarrage.

Pour arrêter l'environnement :

```bash
docker compose down
```

Une collection Postman importable est disponible dans
`BarterSwap.postman_collection.json`. Elle exécute le parcours complet et
enregistre automatiquement les IDs créés dans ses variables de collection.

## Architecture

Le code reste dans un seul package Go, comme demandé dans le sujet :

```text
handler HTTP → service métier → repository SQL → PostgreSQL
```

Les handlers se chargent du JSON et des codes HTTP. Les règles métier sont dans
les services. Les opérations de crédits utilisent des transactions SQL. Le
serveur utilise aussi des middlewares simples pour le logging, le CORS et la
récupération des panics.

## Endpoints utilisateurs

| Méthode | Route | Description | Authentification |
|---|---|---|---|
| `POST` | `/api/users` | Créer un utilisateur avec 10 crédits | Non |
| `GET` | `/api/users/{id}` | Consulter un profil public | Non |
| `PUT` | `/api/users/{id}` | Modifier son profil | `X-User-ID` |
| `GET` | `/api/users/{id}/skills` | Consulter les compétences | Non |
| `PUT` | `/api/users/{id}/skills` | Remplacer les compétences | `X-User-ID` |

## Endpoints services

| Méthode | Route | Description | Authentification |
|---|---|---|---|
| `GET` | `/api/services` | Lister les services actifs | Non |
| `POST` | `/api/services` | Publier un service | `X-User-ID` |
| `GET` | `/api/services/{id}` | Consulter un service | Non |
| `PUT` | `/api/services/{id}` | Modifier son service | `X-User-ID` |
| `DELETE` | `/api/services/{id}` | Supprimer son service | `X-User-ID` |

Filtres disponibles sur `GET /api/services` : `categorie`, `ville` et `search`.
La catégorie doit être une catégorie du sujet et correspondre à une compétence
du fournisseur.

## Endpoints échanges

| Méthode | Route | Description | Authentification |
|---|---|---|---|
| `POST` | `/api/exchanges` | Demander un service | `X-User-ID` |
| `GET` | `/api/exchanges` | Lister ses échanges | `X-User-ID` |
| `GET` | `/api/exchanges/{id}` | Consulter un échange participant | `X-User-ID` |
| `PUT` | `/api/exchanges/{id}/accept` | Accepter et bloquer les crédits | propriétaire |
| `PUT` | `/api/exchanges/{id}/reject` | Refuser une demande | propriétaire |
| `PUT` | `/api/exchanges/{id}/complete` | Terminer et transférer les crédits | participant |
| `PUT` | `/api/exchanges/{id}/cancel` | Annuler et rembourser si nécessaire | participant |

Le cycle normal est `pending → accepted → completed`. Un refus mène à
`rejected`, une annulation à `cancelled`. Un service ne peut avoir qu'une seule
demande `pending` ou `accepted`.

## Endpoints avis et statistiques

| Méthode | Route | Description | Authentification |
|---|---|---|---|
| `POST` | `/api/exchanges/{id}/review` | Laisser un avis après un échange terminé | participant |
| `GET` | `/api/users/{id}/reviews` | Avis reçus par un utilisateur | Non |
| `GET` | `/api/services/{id}/reviews` | Avis liés à un service | Non |
| `GET` | `/api/users/{id}/stats` | Statistiques d'un utilisateur | Non |

Une note est comprise entre 1 et 5. Un utilisateur ne peut laisser qu'un seul
avis par échange, et un avis est possible uniquement après `completed`.

## Exemple rapide

Créer un utilisateur :

```bash
curl -X POST http://localhost:8080/api/users \
  -H 'Content-Type: application/json' \
  -d '{"pseudo":"Alice","bio":"Passionnée de jardinage","ville":"Paris"}'
```

Définir ses compétences :

```bash
curl -X PUT http://localhost:8080/api/users/1/skills \
  -H 'Content-Type: application/json' \
  -H 'X-User-ID: 1' \
  -d '{"skills":[{"nom":"Jardinage","niveau":"expert"}]}'
```

Publier un service :

```bash
curl -X POST http://localhost:8080/api/services \
  -H 'Content-Type: application/json' \
  -H 'X-User-ID: 1' \
  -d '{"titre":"Aide au jardin","description":"Deux heures de jardinage","categorie":"Jardinage","duree_minutes":120,"credits":2,"ville":"Paris"}'
```

Demander puis accepter un échange :

```bash
curl -X POST http://localhost:8080/api/exchanges \
  -H 'Content-Type: application/json' \
  -H 'X-User-ID: 2' \
  -d '{"service_id":1}'

curl -X PUT http://localhost:8080/api/exchanges/1/accept \
  -H 'X-User-ID: 1'
```

Terminer, noter et consulter les statistiques :

```bash
curl -X PUT http://localhost:8080/api/exchanges/1/complete \
  -H 'X-User-ID: 2'

curl -X POST http://localhost:8080/api/exchanges/1/review \
  -H 'Content-Type: application/json' \
  -H 'X-User-ID: 2' \
  -d '{"note":5,"commentaire":"Très bon service"}'

curl http://localhost:8080/api/users/1/reviews
curl http://localhost:8080/api/services/1/reviews
curl http://localhost:8080/api/users/1/stats
```

## Tests et qualité

```bash
docker compose exec go-dev go test -v -cover ./...
docker compose exec go-dev go vet ./...
docker compose exec go-dev gofmt -l *.go
```

## Documentation de l'API

Vous pouvez consulter la documentation interactive de l'API ici : 
[Documentation BarterSwap (OpenAPI)](https://pyracantharia.github.io/swaggerBarterswap/)
