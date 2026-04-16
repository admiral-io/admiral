#!/usr/bin/env bash
set -e

echo "Waiting for Keycloak to be fully ready..."
sleep 30

echo "Configuring Keycloak SSL settings..."

# Use kcadm.sh inside the keycloak container
docker exec keycloak /opt/keycloak/bin/kcadm.sh config credentials \
    --server http://localhost:9090 \
    --realm master \
    --user admiral \
    --password shipitnow

docker exec keycloak /opt/keycloak/bin/kcadm.sh update realms/master \
    -s sslRequired=NONE

echo "SSL requirements disabled successfully!"
