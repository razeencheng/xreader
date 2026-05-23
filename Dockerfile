# Stage 1: Build frontend
FROM node:20-alpine AS frontend
WORKDIR /app/web
COPY web/package.json web/pnpm-lock.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

# Stage 2: Build Go binary
FROM golang:1.25-alpine AS backend
WORKDIR /app/server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
# Copy static export into the embed directory
COPY --from=frontend /app/web/out ./cmd/xreader/static/
RUN CGO_ENABLED=0 GOFLAGS="-trimpath" go build -ldflags="-s -w" -o /xreader ./cmd/xreader

# Stage 3: Minimal runtime
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -H xreader
COPY --from=backend /xreader /usr/local/bin/xreader
COPY server/db/migrations /migrations
USER xreader
EXPOSE 3000
ENTRYPOINT ["xreader"]
