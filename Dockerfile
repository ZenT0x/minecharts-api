# Step 1 : Build
FROM golang:1.20-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o minecharts .

# Step 2 : Execution
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/minecharts .
EXPOSE 8080
CMD ["./minecharts"]
