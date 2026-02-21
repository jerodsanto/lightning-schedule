#!/bin/bash

# Deploy script to upload dist directory to remote server

set -e

# Configuration
HOST="dh"
BINARY="lightning-schedule"
WEB_DIR="~/schedule.omahalightningbasketball.com"
SCRIPT_DIR="~/scripts"

# Compile Linux binary
echo "🔨 Compiling Linux binary..."
GOOS=linux GOARCH=amd64 go build -o ${BINARY}

# Upload binary to remote scripts directory
echo "📤 Uploading binary to ${HOST}:${SCRIPT_DIR}..."
scp -q ${BINARY} ${HOST}:${SCRIPT_DIR}

# Upload static files to web directory
echo "📁 Uploading static files to ${HOST}:${WEB_DIR}..."
scp -r -q static/* ${HOST}:${WEB_DIR}/

# Execute the binary remotely
echo "🚀 Executing binary on ${HOST}..."
ssh ${HOST} "${SCRIPT_DIR}/${BINARY} ${WEB_DIR}"

# Delete local binary
echo "🗑️  Removing local binary..."
rm ${BINARY}

echo "✅ Deploy complete!"
