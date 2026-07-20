#!/bin/bash

# Deploy script: uploads the binary once, then per-program static files,
# and runs the generator remotely for each program.
# Usage: ./deploy.sh [program]   (no arg = all programs in PROGRAMS)

set -e

HOST="dh"
BINARY="lightning-schedule"
SCRIPT_DIR="~/scripts"

# Programs to deploy. Add "warriors" once its domain and web dir exist.
PROGRAMS=("lightning")

# macOS ships bash 3.2 (no associative arrays), hence the case statement
web_dir() {
  case "$1" in
    lightning) echo "~/schedule.omahalightningbasketball.com" ;;
    # warriors) echo "~/schedule.WARRIORS-DOMAIN-HERE" ;;
    *) echo "" ;;
  esac
}

if [ -n "$1" ]; then
  PROGRAMS=("$1")
fi

# Compile Linux binary
echo "🔨 Compiling Linux binary..."
GOOS=linux GOARCH=amd64 go build -o ${BINARY}

# Upload binary to remote scripts directory
echo "📤 Uploading binary to ${HOST}:${SCRIPT_DIR}..."
scp -q ${BINARY} ${HOST}:${SCRIPT_DIR}

for prog in "${PROGRAMS[@]}"; do
  WEB_DIR=$(web_dir "$prog")
  if [ -z "$WEB_DIR" ]; then
    echo "❌ No web dir configured for ${prog} (edit web_dir() in deploy.sh)"
    exit 1
  fi

  echo "📁 Uploading static files to ${HOST}:${WEB_DIR}..."
  scp -r -q static/${prog}/* ${HOST}:${WEB_DIR}/

  echo "🚀 Generating ${prog} on ${HOST}..."
  ssh ${HOST} "${SCRIPT_DIR}/${BINARY} -program ${prog} ${WEB_DIR}"
done

# Delete local binary
echo "🗑️  Removing local binary..."
rm ${BINARY}

echo "✅ Deploy complete!"
