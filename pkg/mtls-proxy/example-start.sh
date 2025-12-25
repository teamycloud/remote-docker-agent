#!/bin/bash
# Example startup script for mTLS proxy

# Configuration
LISTEN_ADDR=":8443"
CA_CERTS="/etc/mtls-proxy/ca1.pem,/etc/mtls-proxy/ca2.pem"
SERVER_CERT="/etc/mtls-proxy/server.crt"
SERVER_KEY="/etc/mtls-proxy/server.key"
ISSUER="tinyscale.com"

# Database configuration
DB_HOST="localhost"
DB_PORT="5432"
DB_USER="tinyscale"
DB_PASSWORD="tinyscale"
DB_NAME="tinyscale-ssh"

# Logging
LOG_LEVEL="info"

# Start the proxy
./bin/connector \
  --listen="$LISTEN_ADDR" \
  --ca-certs="$CA_CERTS" \
  --server-cert="$SERVER_CERT" \
  --server-key="$SERVER_KEY" \
  --issuer="$ISSUER" \
  --db-host="$DB_HOST" \
  --db-port="$DB_PORT" \
  --db-user="$DB_USER" \
  --db-password="$DB_PASSWORD" \
  --db-name="$DB_NAME" \
  --log-level="$LOG_LEVEL"
