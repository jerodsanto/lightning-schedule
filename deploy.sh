#!/bin/bash

# Deploy script to upload dist directory to remote server
# Uploads to mydh host at ~/testing.jerodsanto.net

set -e

echo "📦 Deploying to mydh:~/schedule.omahalightningbasketball.com..."

# Check if dist directory exists
if [ ! -d "dist" ]; then
    echo "❌ Error: dist directory not found. Run 'npm start' first to generate the files."
    exit 1
fi

# Use rsync to upload the dist directory contents
rsync -avz --delete dist/ mydh:~/schedule.omahalightningbasketball.com/

echo "✅ Deploy complete!"
echo "🌐 Your site should be available at your configured domain."
