SRC=templates
DST=static
ICON=icon.png
FAVICON=favicon.png

# Generate different sizes from a source image
magick ${SRC}/${ICON} -resize 120x120 ${DST}/apple-touch-icon-120x120.png
magick ${SRC}/${ICON} -resize 152x152 ${DST}/apple-touch-icon-152x152.png
magick ${SRC}/${ICON} -resize 180x180 ${DST}/apple-touch-icon.png

magick ${SRC}/${ICON} -resize 192x192 ${DST}/android-chrome-192x192.png
magick ${SRC}/${ICON} -resize 512x512 ${DST}/android-chrome-512x512.png

# Create .ico file with multiple resolutions
magick ${SRC}/${FAVICON} -resize 16x16 ${DST}/favicon-16x16.png
magick ${SRC}/${FAVICON} -resize 32x32 ${DST}/favicon-32x32.png
magick ${SRC}/${FAVICON} -define icon:auto-resize=64,48,32,16 ${DST}/favicon.ico

# Symlink static files to dist (only if they don't exist)
for file in static/*; do
  filename=$(basename "$file")
  if [ ! -e "dist/$filename" ]; then
    ln -s "../static/$filename" "dist/$filename"
  fi
done

go run generate.go
