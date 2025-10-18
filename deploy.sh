#!/bin/bash

# Deploy script to upload dist directory to remote server

set -e

# Configuration
REMOTE_HOST="mydh"

echo "📦 Deploying binary to ${REMOTE_HOST}"

# Compile Linux binary
echo "🔨 Compiling Linux binary..."
GOOS=linux GOARCH=amd64 go build -o generate-linux

# Upload binary to remote scripts directory
echo "📤 Uploading binary to ${REMOTE_HOST}:~/scripts..."
scp generate-linux ${REMOTE_HOST}:~/scripts/

# Delete local binary
echo "🗑️  Removing local binary..."
rm generate-linux

echo "✅ Deploy complete!"
