# syntax=docker/dockerfile:1

# --- Stage 1: build the Vue frontend ---------------------------------------
FROM node:20-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package.json ./
RUN npm install
COPY frontend/ ./
RUN npm run build

# --- Stage 2: build the Go backend (static, pure-Go SQLite) ----------------
FROM golang:1.25-alpine AS backend
RUN apk add --no-cache git
WORKDIR /app/backend
COPY backend/ ./
# Embed the freshly built frontend into the binary.
RUN rm -rf internal/web/dist && mkdir -p internal/web/dist
COPY --from=frontend /app/frontend/dist ./internal/web/dist
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /dashboard .

# --- Stage 3: minimal runtime image ----------------------------------------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates && mkdir -p /data
COPY --from=backend /dashboard /usr/local/bin/dashboard
ENV DB_PATH=/data/dashboard.db \
    PORT=8080 \
    HOST_SYS=/host/sys \
    HOST_PROC=/host/proc
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/dashboard"]
