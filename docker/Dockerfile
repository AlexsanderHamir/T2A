# Production Hamix image — API binary + built SPA assets; git for gitwork.
FROM node:20-bookworm AS build-web

WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.25-bookworm AS build-api

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/taskapi ./cmd/taskapi

FROM debian:12-slim AS runtime

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    git \
    && rm -rf /var/lib/apt/lists/*

RUN useradd -r -u 10001 -d /app -s /usr/sbin/nologin appuser

WORKDIR /app
COPY --from=build-api /out/taskapi /app/taskapi
COPY --from=build-web /app/web/dist /app/web/dist

USER appuser

EXPOSE 8080
ENV HAMIX_LISTEN_HOST=0.0.0.0

ENTRYPOINT ["/app/taskapi"]
