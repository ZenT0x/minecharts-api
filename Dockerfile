# Stage 1: Build the static binary
FROM golang:1.24-alpine AS builder
WORKDIR /app
ENV CGO_ENABLED=0

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

RUN go install github.com/swaggo/swag/cmd/swag@latest
RUN swag init -g cmd/main.go -o cmd/docs

RUN --mount=type=cache,target=/root/.cache/go-build go build -ldflags="-s -w" -o build/minecharts ./cmd

# Stage 2: Create the minimal image using scratch
FROM alpine:latest
WORKDIR /app

RUN mkdir -p /app/data && chmod 777 /app/data

COPY --from=builder /app/build/minecharts .

EXPOSE 8080
ENTRYPOINT ["./minecharts"]
