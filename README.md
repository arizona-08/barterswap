# BarterSwap — mini setup

Ce squelette respecte les premières contraintes de l'énoncé : Go, bibliothèque
standard uniquement, un seul package et séparation entre exposition HTTP
(`handlers.go`) et logique métier (`service.go`). Il ne branche pas encore la
base de données.


L'API écoute par défaut sur `http://localhost:8080`. La variable `PORT` permet
de changer le port.

## Environnement de développement Docker

```bash
# pour construire l'image
docker compose up --build -d
```

```bash
# pour compiler et lancer l'API
docker exec -it go-dev go run .
```

```bash
# pour compiler seulement
# important de mettre le code complilé dans le dossier build pour qu'il reste dans le .gitignore
docker exec -it go-dev go build -o build/
```

## Routes d'exemple

| Méthode | Route | Description |
|---|---|---|
| `GET` | `/api/test` | Vérifie que l'API répond |
| `PATCH` | `/api/test` | Montre la lecture et la validation d'un JSON |

```bash
curl http://localhost:8080/api/test
```

```bash
curl -X PATCH http://localhost:8080/api/test \
  -H 'Content-Type: application/json' \
  -d '{"message":"Hello BarterSwap"}'
```

Le `PATCH` est volontairement sans persistance : il sert uniquement de modèle
pour les futures routes. L'équipe peut reprendre le même découpage modèle,
service et handler pour chaque domaine métier.

## Tests

```bash
go test ./...
```
