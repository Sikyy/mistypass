# syntax=docker/dockerfile:1.7

FROM golang:1.25.4-bookworm@sha256:a62429958d6ce7c3a5314e3996d0a19eb9087e25e2671658a04b9f8ba91def42 AS builder

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
