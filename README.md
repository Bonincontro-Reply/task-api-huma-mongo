# Task API (Huma + MongoDB)

Microservizio per task con API Huma, storage MongoDB e frontend statico (Nginx).

## Struttura

```
.
|-- cmd/
|-- internal/
|-- frontend/
|-- deploy/
|   |-- docker/
|   |   |-- api.Dockerfile
|   |   |-- frontend.Dockerfile
|   |   `-- docker-compose.yml
|   `-- helm/
|       `-- task-api-huma-mongo/
|-- scripts/
|   |-- start-kind.cmd
|   `-- cleanup-kind.cmd
`-- openapi.json
```

## Requisiti

- Go 1.23+ (solo per esecuzione locale)
- Docker Desktop
- Helm + kubectl (per Kubernetes)
- kind (per gli script Windows)

Verifica rapida (facoltativa):

```powershell
go version
docker version
helm version
kubectl version --client
kind version
```

## Configurazione (env)

- `PORT` (default `8080`)
- `MONGODB_URI` (default `mongodb://localhost:27017`)
- `MONGODB_DB` (default `taskdb`)
- `MONGODB_COLLECTION` (default `tasks`)
- `CORS_ALLOW_ORIGINS` (default `http://localhost:8081,http://127.0.0.1:8081`)

## Quick start (Docker Compose) - consigliato

```powershell
cd C:\Progetti\task-api-huma-mongo
docker compose -f deploy/docker/docker-compose.yml up --build
```

Frontend: `http://localhost:8081` (proxy API su `/api`)

Stop e pulizia:

```powershell
cd C:\Progetti\task-api-huma-mongo
docker compose -f deploy/docker/docker-compose.yml down -v
```

## Quick start (kind + Helm, Windows)

```powershell
cd C:\Progetti\task-api-huma-mongo
scripts\start-kind.cmd
```

Pulizia:

```powershell
cd C:\Progetti\task-api-huma-mongo
scripts\cleanup-kind.cmd
```

## Helm (manuale)

```powershell
cd C:\Progetti\task-api-huma-mongo
helm upgrade --install task-api-huma-mongo .\deploy\helm\task-api-huma-mongo --namespace task-api --create-namespace
```

Port-forward:

```powershell
kubectl -n task-api port-forward svc/task-api-huma-mongo-frontend 8081:80
kubectl -n task-api port-forward svc/task-api-huma-mongo-api 8080:8080
```

Valori configurabili in `deploy/helm/task-api-huma-mongo/values.yaml`.

## Avvio locale (API only)

Avvia MongoDB con Docker:

```powershell
cd C:\Progetti\task-api-huma-mongo
docker run --rm -d --name task-mongo -p 27017:27017 -e MONGODB_ALLOW_EMPTY_PASSWORD=yes -e MONGODB_DATABASE=taskdb bitnami/mongodb:latest
```

Avvia API in locale:

```powershell
cd C:\Progetti\task-api-huma-mongo
go mod tidy
go run ./cmd/server
```

Stop MongoDB:

```powershell
docker stop task-mongo
```

## API (test rapido)

Health:

```powershell
curl http://localhost:8080/health
```

Create task:

```powershell
curl -X POST http://localhost:8080/tasks -H "Content-Type: application/json" -d "{\"title\":\"Buy milk\",\"tags\":[\"home\",\"errand\"]}"
```

Spec OpenAPI: `openapi.json`
