#!/bin/bash

# Deploy script to upload dist directory to remote server

set -e

# Configuration
DOMAIN="schedule.omahalightningbasketball.com"
# DOMAIN="testing.jerodsanto.net"
REMOTE_HOST="mydh"

echo "📦 Deploying to ${REMOTE_HOST}:~/${DOMAIN}..."

# Check if dist directory exists
if [ ! -d "dist" ]; then
    echo "❌ Error: dist directory not found. Run 'npm start' first to generate the files."
    exit 1
fi

# Use rsync to upload the dist directory contents
rsync -avz --delete dist/ ${REMOTE_HOST}:~/${DOMAIN}/

echo "✅ Deploy complete!"
echo "🌐 Your site should be available at https://${DOMAIN}"
