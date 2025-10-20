#!/bin/bash

# Deploy script to upload dist directory to remote server

set -e

# Configuration
REMOTE_HOST="mydh"
BINARY="lightning-schedule"

# Compile Linux binary
echo "🔨 Compiling Linux binary..."
GOOS=linux GOARCH=amd64 go build -o ${BINARY}

# Upload binary to remote scripts directory
echo "📤 Uploading binary to ${REMOTE_HOST}:~/scripts..."
scp -q ${BINARY} ${REMOTE_HOST}:~/scripts/

# Execute the binary remotely
echo "🚀 Executing binary on ${REMOTE_HOST}..."
ssh ${REMOTE_HOST} "~/scripts/${BINARY} ~/schedule.omahalightningbasketball.com"

# Delete local binary
echo "🗑️  Removing local binary..."
rm ${BINARY}

echo "✅ Deploy complete!"
