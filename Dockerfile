# syntax=docker/dockerfile:1

##
## Frontend build stage
##
FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend

# Install dependencies first (better layer cache)
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

# Build frontend (outputs to ../backend/static via vite.config.ts)
COPY frontend/ ./
RUN mkdir -p /app/backend/static
RUN npm run build

##
## Backend build stage
##
FROM golang:1.21-alpine AS backend-builder
WORKDIR /app/backend

# Build dependencies (CGO is required for sqlite driver)
RUN apk add --no-cache build-base

# Download Go modules first (better layer cache)
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Copy source and embed built frontend assets
COPY backend/ ./
COPY --from=frontend-builder /app/backend/static ./static

# Build a smaller binary
RUN go build -trimpath -ldflags="-s -w" -o /app/aca .

##
## Runtime stage
##
FROM alpine:3.19

# Runtime deps:
# - ca-certificates: HTTPS requests
# - bash/tmux: terminal hosting features
RUN apk add --no-cache ca-certificates bash tmux

RUN addgroup -S aca && adduser -S aca -G aca
WORKDIR /app

COPY --from=backend-builder /app/aca /app/aca
RUN mkdir -p /app/data && chown -R aca:aca /app

USER aca
EXPOSE 34007
ENV SERVER_HOST=0.0.0.0
ENV SERVER_PORT=34007
CMD ["/app/aca"]
