# syntax=docker/dockerfile:1.6

# stage 1: frontend build
FROM node:20-alpine AS web
WORKDIR /web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# stage 2: go build
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
# vite writes to ../internal/web/dist from /web, i.e. /internal/web/dist in the web stage.
COPY --from=web /internal/web/dist ./internal/web/dist
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/bookshelf ./cmd/bookshelf

# stage 3: runtime
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/bookshelf /usr/local/bin/bookshelf
USER nonroot:nonroot
EXPOSE 19320
ENTRYPOINT ["/usr/local/bin/bookshelf"]
