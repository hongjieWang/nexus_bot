#!/bin/bash

# Exit immediately if a command exits with a non-zero status
set -e

echo "🚀 Building bot for linux/amd64 (x86_64)..."
GOOS=linux GOARCH=amd64 go build -o bot_linux_amd64

echo "📦 Packaging into bot_deploy_x86_64.tar.gz..."
tar -czvf bot_deploy_x86_64.tar.gz bot_linux_amd64 .env strategy/*.go

echo "✅ Build and package complete!"
