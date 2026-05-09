# syntax=docker/dockerfile:1.7

FROM golang:1.25.10-bookworm@sha256:e3a54b77385b4f8a31c1db4d12429ffb3718ea76865731a787c497755d409547 AS builder

WORKDIR /src
COPY api/go.mod api/go.sum ./api/
WORKDIR /src/api
RUN go mod download

COPY api/ ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/mistypass-api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=builder /out/mistypass-api /app/mistypass-api

EXPOSE 8080
USER nonroot:nonroot

ENTRYPOINT ["/app/mistypass-api"]
