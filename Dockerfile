# Step 1 : Build
FROM golang:1.20-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o build/minecharts ./cmd

# Step 2 : Execution
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/build/minecharts .
EXPOSE 8080
CMD ["./minecharts"]
