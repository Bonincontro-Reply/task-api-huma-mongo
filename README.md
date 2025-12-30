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
|   |-- examples/
|   |   `-- taskseed-sample.yaml
|   `-- helm/
|       `-- task-api-huma-mongo/
|-- scripts/
|   |-- start-kind.cmd
|   `-- cleanup-kind.cmd
`-- openapi.json
```

## Requisiti

- Go 1.25+ (solo per esecuzione locale)
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

Lo script applica anche l'esempio `deploy/examples/taskseed-sample.yaml` (se presente).

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

## Seeding DB (TaskSeed CRD)

La chart installa il CRD `TaskSeed` e un controller che crea Job di seeding.
Esempio (vedi `deploy/examples/taskseed-sample.yaml`):

```yaml
apiVersion: tasks.huma.io/v1alpha1
kind: TaskSeed
metadata:
  name: sample-seed
  namespace: task-api
spec:
  size: 50
  seed: 1
  mode: upsert
  seedVersion: "sample-v1"
  titlePrefix: "Sample"
  tags:
    - sample
    - seed
  doneRatio: 0.25
  tagCountMin: 0
  tagCountMax: 2
  database: taskdb
  collection: tasks
  maintenanceSchedule: "* * * * *"
```

Se vuoi usare un URI Mongo in Secret:

```yaml
spec:
  mongodb:
    uriSecretRef:
      name: mongo-conn
      key: uri
```

Il controller usa l'immagine API per eseguire `/app/seeder` e non tocca ambienti prod a meno che tu non crei il CRD.
Se imposti `maintenanceSchedule`, ogni esecuzione elimina le task completate e crea nuove task per tornare al numero `spec.size`.

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
