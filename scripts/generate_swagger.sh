#!/bin/bash

# Ensure swag is installed
if ! command -v swag &> /dev/null; then
    echo "Installing swag CLI..."
    go install github.com/swaggo/swag/cmd/swag@latest
fi

# Run swag to generate Swagger documentation
echo "Generating Swagger docs..."
cd /home/zentox/Bureau/minecharts
swag init -g cmd/main.go -o cmd/docs

echo "Swagger documentation generated in cmd/docs/"
