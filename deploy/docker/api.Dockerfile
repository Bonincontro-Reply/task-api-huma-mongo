FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/seeder ./cmd/seeder
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/seed-controller ./cmd/seed-controller

FROM gcr.io/distroless/static:nonroot

WORKDIR /app
COPY --from=build /out/server /app/server
COPY --from=build /out/seeder /app/seeder
COPY --from=build /out/seed-controller /app/seed-controller

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/server"]
