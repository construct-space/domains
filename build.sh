#!/bin/sh
set -e

echo "==> Building Vue UI..."
cd ui && npm run build && cd ..

echo "==> Building Go binary..."
CGO_ENABLED=0 go build -ldflags="-s -w" -o domains .
ls -lh domains
echo "==> Done! Run with: ./domains"
