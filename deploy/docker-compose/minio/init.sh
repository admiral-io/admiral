#!/usr/bin/env bash
set -e

# Set alias
mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD"

# Create buckets
mc mb -p local/manifests 2>/dev/null || true
mc mb -p local/revisions 2>/dev/null || true

# Output confirmation
echo "MinIO buckets and static user setup completed"
