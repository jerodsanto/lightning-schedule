#!/bin/bash

# Build per-program assets (icons, manifest) into static/<program>/,
# symlink them into dist/<program>/, and generate schedules.

set -e

for progdir in programs/*/; do
  prog=$(basename "$progdir")
  SRC="programs/${prog}"
  DST="static/${prog}"
  mkdir -p "$DST"

  # Generate icon sizes from source images (skip if sources are missing or unchanged)
  if [ -f "${SRC}/icon.png" ]; then
    if [ "${SRC}/icon.png" -nt "${DST}/apple-touch-icon.png" ]; then
      magick "${SRC}/icon.png" -resize 120x120 "${DST}/apple-touch-icon-120x120.png"
      magick "${SRC}/icon.png" -resize 152x152 "${DST}/apple-touch-icon-152x152.png"
      magick "${SRC}/icon.png" -resize 180x180 "${DST}/apple-touch-icon.png"
      magick "${SRC}/icon.png" -resize 192x192 "${DST}/android-chrome-192x192.png"
      magick "${SRC}/icon.png" -resize 512x512 "${DST}/android-chrome-512x512.png"
    fi
  else
    echo "⚠️  ${SRC}/icon.png missing — skipping icon generation for ${prog}"
  fi

  if [ -f "${SRC}/favicon.png" ]; then
    if [ "${SRC}/favicon.png" -nt "${DST}/favicon-16x16.png" ]; then
      magick "${SRC}/favicon.png" -resize 16x16 "${DST}/favicon-16x16.png"
      magick "${SRC}/favicon.png" -resize 32x32 "${DST}/favicon-32x32.png"
      magick "${SRC}/favicon.png" -define icon:auto-resize=64,48,32,16 "${DST}/favicon.ico"
    fi
  else
    echo "⚠️  ${SRC}/favicon.png missing — skipping favicon generation for ${prog}"
  fi

  # Generate manifest.json from the program config
  jq '{
    name: (.name + " Schedule"),
    short_name: .name,
    icons: [
      {src: "/android-chrome-192x192.png", sizes: "192x192", type: "image/png", purpose: "any maskable"},
      {src: "/android-chrome-512x512.png", sizes: "512x512", type: "image/png", purpose: "any maskable"}
    ],
    theme_color: .themeColor,
    background_color: "#f5f5f5",
    display: "standalone",
    start_url: "/"
  }' "${SRC}/config.json" > "${DST}/manifest.json"

  # Symlink static files into dist/<program> (only if they don't exist)
  mkdir -p "dist/${prog}"
  for file in "${DST}"/*; do
    filename=$(basename "$file")
    if [ ! -e "dist/${prog}/${filename}" ]; then
      ln -s "../../static/${prog}/${filename}" "dist/${prog}/${filename}"
    fi
  done

  # Generate the schedule (a program with an unfilled config fails without stopping the others)
  go run generate.go -program "$prog" || echo "⚠️  generate failed for ${prog}"
done
