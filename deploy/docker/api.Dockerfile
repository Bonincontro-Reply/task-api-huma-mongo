FROM golang:1.23-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM gcr.io/distroless/static:nonroot

WORKDIR /app
COPY --from=build /out/server /app/server

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/server"]
