#!/usr/bin/env bash
set -e

# Set alias
mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD"

# Create buckets
mc mb -p local/admiral 2>/dev/null || true

# Output confirmation
echo "MinIO buckets setup completed"
