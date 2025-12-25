# Quick Start Guide - mTLS Proxy

This guide will help you get the mTLS proxy running quickly for testing.

## Prerequisites

1. PostgreSQL database (compatible with ssh-router schema)
2. Certificates (CA, server, and client with SPIFFE URI)
3. Go 1.24+ (for building)

## Step 1: Build

```bash
cd /Users/jijiechen/go/src/github.com/teamycloud/tsctl
go build -o bin/connector ./cmd/connector
go build -o bin/mtls-test-client ./pkg/mtls-proxy/example-client
```

## Step 2: Set Up Database

### Create Tables

```sql
-- Backend hosts
CREATE TABLE IF NOT EXISTS backend_hosts (
    connect_id       TEXT PRIMARY KEY,
    internal_ip_addr TEXT NOT NULL,
    org_id           TEXT NOT NULL,
    user_ids         TEXT[] NOT NULL DEFAULT '{}',
    team_ids         TEXT[] NOT NULL DEFAULT '{}',
    host_user        TEXT
);

-- Teams
CREATE TABLE IF NOT EXISTS teams (
    team_id   TEXT PRIMARY KEY,
    org_id    TEXT NOT NULL,
    user_ids  TEXT[] NOT NULL DEFAULT '{}'
);

-- Users
CREATE TABLE IF NOT EXISTS users (
    user_id     TEXT PRIMARY KEY,
    org_id      TEXT NOT NULL,
    public_keys BYTEA[] NOT NULL DEFAULT '{}'
);
```

### Add Test Data

```sql
-- Add test organization
INSERT INTO users (user_id, org_id, public_keys)
VALUES ('user-456', 'org-123', ARRAY[]::BYTEA[]);

-- Add test host
INSERT INTO backend_hosts (connect_id, internal_ip_addr, org_id, user_ids, team_ids)
VALUES ('test-host-01', '127.0.0.1:8080', 'org-123', ARRAY['user-456'], ARRAY[]);
```

## Step 3: Generate Certificates

### Quick Test Certificates (OpenSSL)

```bash
# Create certificates directory
mkdir -p certs
cd certs

# Generate CA
openssl genrsa -out ca.key 4096
openssl req -new -x509 -days 3650 -key ca.key -out ca.pem \
  -subj "/C=US/ST=CA/L=SF/O=Test/CN=Test CA"

# Generate server certificate
openssl genrsa -out server.key 2048
openssl req -new -key server.key -out server.csr \
  -subj "/C=US/ST=CA/L=SF/O=Test/CN=localhost"

cat > server.ext << EOF
subjectAltName = DNS:localhost,IP:127.0.0.1
extendedKeyUsage = serverAuth
EOF

openssl x509 -req -days 365 -in server.csr \
  -CA ca.pem -CAkey ca.key -CAcreateserial \
  -out server.crt -extfile server.ext

# Generate client certificate with SPIFFE URI
openssl genrsa -out client.key 2048
openssl req -new -key client.key -out client.csr \
  -subj "/C=US/ST=CA/L=SF/O=Test/CN=user-456"

cat > client.ext << EOF
subjectAltName = URI:spiffe://tinyscale.com/orgs/org-123/users/user-456
extendedKeyUsage = clientAuth
EOF

openssl x509 -req -days 365 -in client.csr \
  -CA ca.pem -CAkey ca.key -CAcreateserial \
  -out client.crt -extfile client.ext

# Verify SPIFFE URI
openssl x509 -in client.crt -text -noout | grep -A 1 "Subject Alternative Name"

cd ..
```

## Step 4: Start a Test Backend Server

```bash
# Simple echo server for testing
nc -l 8080
# Or use Python:
python3 -m http.server 8080
```

## Step 5: Start the mTLS Proxy

```bash
./bin/connector \
  --listen=":8443" \
  --ca-certs="certs/ca.pem" \
  --server-cert="certs/server.crt" \
  --server-key="certs/server.key" \
  --issuer="tinyscale.com" \
  --db-host="localhost" \
  --db-port=5432 \
  --db-user="tinyscale" \
  --db-password="tinyscale" \
  --db-name="tinyscale-ssh" \
  --log-level="info"
```

Expected output:
```
INFO[...] mTLS proxy listening on :8443
INFO[...] mTLS proxy started successfully
```

## Step 6: Test with Client

```bash
./bin/mtls-test-client \
  --proxy="localhost:8443" \
  --cert="certs/client.crt" \
  --key="certs/client.key" \
  --ca="certs/ca.pem" \
  --connect-id="test-host-01"
```

Expected output:
```
Connecting to proxy at localhost:8443...
Connected. Sending connect_id: test-host-01
Proxy response: OK
Connection established successfully!
You can now communicate with the backend server
Type messages to send to backend (Ctrl+C to exit)
```

## Verification Steps

### 1. Check Proxy Logs

Proxy should show:
```
INFO[...] authenticated user: user-456 (org: org-123)
INFO[...] routing connection to: test-host-01
INFO[...] routing user user-456 to backend 127.0.0.1:8080
```

### 2. Send Data Through Proxy

Type in the client terminal:
```
Hello, backend!
```

The backend (nc/Python server) should receive and echo this message.

### 3. Test Authorization Failure

Try with invalid connect_id:
```bash
./bin/mtls-test-client \
  --proxy="localhost:8443" \
  --cert="certs/client.crt" \
  --key="certs/client.key" \
  --ca="certs/ca.pem" \
  --connect-id="invalid-host"
```

Should see:
```
Proxy returned error: ERROR: routing failed: ...
```

## Troubleshooting

### "Certificate verification failed"

- Check that client cert is signed by CA in `--ca-certs`
- Verify certificate is not expired: `openssl x509 -in client.crt -noout -dates`
- Verify SPIFFE URI: `openssl x509 -in client.crt -text -noout | grep URI`

### "No valid SPIFFE URI found"

- Regenerate client certificate with proper SAN extension
- Ensure URI format: `spiffe://tinyscale.com/orgs/org-123/users/user-456`

### "Authorization failed"

- Check database: `SELECT * FROM users WHERE user_id = 'user-456'`
- Verify org_id matches: `SELECT * FROM backend_hosts WHERE connect_id = 'test-host-01'`
- Confirm user in user_ids array

### "Failed to connect to database"

- Verify PostgreSQL is running: `psql -h localhost -U tinyscale -d tinyscale-ssh`
- Check connection parameters
- Verify database exists and tables are created

### "Failed to connect to backend"

- Verify backend is running on specified port
- Check firewall rules
- Verify internal_ip_addr in database is correct

## Next Steps

1. **Production Certificates**: Use proper PKI (SPIRE, step-ca)
2. **Monitoring**: Add Prometheus metrics
3. **Logging**: Configure log aggregation
4. **High Availability**: Run multiple proxy instances
5. **Load Balancing**: Use HAProxy or nginx in front
6. **Database**: Configure connection pooling and replication

## Configuration Options

All command-line flags can be set via environment variables or config file.

### Environment Variables

```bash
export MTLS_PROXY_LISTEN=":8443"
export MTLS_PROXY_CA_CERTS="certs/ca.pem"
export MTLS_PROXY_SERVER_CERT="certs/server.crt"
export MTLS_PROXY_SERVER_KEY="certs/server.key"
export MTLS_PROXY_ISSUER="tinyscale.com"
export MTLS_PROXY_DB_HOST="localhost"
export MTLS_PROXY_DB_PORT="5432"
export MTLS_PROXY_DB_USER="tinyscale"
export MTLS_PROXY_DB_PASSWORD="tinyscale"
export MTLS_PROXY_DB_NAME="tinyscale-ssh"
export MTLS_PROXY_LOG_LEVEL="info"
```

(Note: Environment variable support would need to be added to main.go)

## Docker Deployment

```dockerfile
FROM golang:1.24 AS builder
WORKDIR /app
COPY . .
RUN go build -o connector ./cmd/connector

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/connector .
EXPOSE 8443
CMD ["./connector"]
```

## Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mtls-proxy
spec:
  replicas: 3
  selector:
    matchLabels:
      app: mtls-proxy
  template:
    metadata:
      labels:
        app: mtls-proxy
    spec:
      containers:
      - name: mtls-proxy
        image: mtls-proxy:latest
        args:
        - --listen=:8443
        - --ca-certs=/etc/certs/ca.pem
        - --server-cert=/etc/certs/server.crt
        - --server-key=/etc/certs/server.key
        - --issuer=tinyscale.com
        - --db-host=postgres.default.svc.cluster.local
        - --db-port=5432
        - --db-user=tinyscale
        - --db-password=$(DB_PASSWORD)
        - --db-name=tinyscale-ssh
        env:
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: postgres-secret
              key: password
        ports:
        - containerPort: 8443
        volumeMounts:
        - name: certs
          mountPath: /etc/certs
          readOnly: true
      volumes:
      - name: certs
        secret:
          secretName: mtls-proxy-certs
---
apiVersion: v1
kind: Service
metadata:
  name: mtls-proxy
spec:
  selector:
    app: mtls-proxy
  ports:
  - protocol: TCP
    port: 8443
    targetPort: 8443
  type: LoadBalancer
```

## Support

For issues or questions:
1. Check the full documentation: `pkg/mtls-proxy/README.md`
2. Review certificate guide: `pkg/mtls-proxy/CERTIFICATE_GUIDE.md`
3. Check implementation details: `MTLS_PROXY_IMPLEMENTATION.md`
