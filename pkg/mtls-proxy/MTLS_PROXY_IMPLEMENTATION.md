# mTLS Proxy Implementation Summary

## Overview

Successfully implemented an mTLS proxy that provides certificate-based authentication and PostgreSQL-driven routing for TCP connections. The implementation is located in `cmd/connector/main.go` with the core logic in `pkg/mtls-proxy/`.

## Implementation Details

### 1. Core Components

#### `pkg/mtls-proxy/config.go`
- Configuration structures for proxy and database
- Support for multiple CA certificates (for CA rotation)
- Validation and default configuration
- Connection string generation for PostgreSQL

#### `pkg/mtls-proxy/identity.go`
- Certificate validation (expiry, signature)
- SPIFFE URI parsing from certificate SAN
- User identity extraction (user_id, org_id, issuer)
- Issuer validation against expected domain
- Format: `spiffe://tinyscale.com/orgs/<org-id>/users/<user-id>`

#### `pkg/mtls-proxy/database.go`
- PostgreSQL connection pool management
- Database queries for backend hosts and routing
- Authorization logic matching ssh-router pattern:
  - Organization-based isolation (org_id)
  - Direct user authorization (user_ids)
  - Team-based authorization (team_ids)
- Backend host lookup by connect_id

#### `pkg/mtls-proxy/proxy.go`
- mTLS TCP proxy server
- TLS connection handling with client certificate verification
- Connection routing based on database queries
- Bidirectional TCP proxying to backend servers
- Graceful shutdown support

#### `cmd/connector/main.go`
- Command-line interface with comprehensive flags
- Integration of all components
- Logging configuration
- Signal handling for graceful shutdown

### 2. Features Implemented

✅ **mTLS Certificate Validation**
- Validates client certificates against configured CA(s)
- Supports multiple CAs for certificate rotation
- Checks certificate expiry and signature
- Uses standard Go crypto/tls and crypto/x509 packages

✅ **SPIFFE Identity Extraction**
- Parses SPIFFE URI from certificate SAN field
- Extracts user_id and org_id from URI path
- Validates issuer domain matches expected value
- Format: `URI:spiffe://tinyscale.com/orgs/<org-id>/users/<user-id>`

✅ **PostgreSQL Database Integration**
- Uses jackc/pgx/v5 for efficient database operations
- Connection pooling with configurable parameters
- Compatible with ssh-router database schema
- Queries `backend_hosts`, `teams`, and `users` tables

✅ **Authorization Logic**
- Identical pattern to ssh-router:
  1. User and host must be in same organization
  2. User must be explicitly authorized via:
     - Direct authorization: user_id in backend_hosts.user_ids
     - Team membership: user_id in team that's in backend_hosts.team_ids
- Only routes connections when authorization succeeds

✅ **Dynamic Routing**
- Queries database for backend server address
- Uses connect_id sent by client to lookup target
- Routes to internal_ip_addr from backend_hosts table
- Returns error if user not authorized or host not found

✅ **TCP Proxy**
- Accepts mTLS connections
- Validates certificates
- Authenticates users
- Routes to appropriate backend
- Bidirectional data transfer
- Graceful connection handling

### 3. Protocol

**Connection Flow:**
1. Client establishes mTLS connection with valid certificate
2. Proxy validates certificate against CA pool
3. Proxy extracts user identity from SPIFFE URI
4. Client sends `<connect_id>\n`
5. Proxy queries database for authorization and routing
6. If authorized:
   - Proxy responds with `OK\n`
   - Proxy connects to backend server
   - Bidirectional data transfer begins
7. If not authorized:
   - Proxy responds with `ERROR: <message>\n`
   - Connection closes

### 4. Database Schema

Compatible with ssh-router schema:

```sql
-- Backend hosts with routing information
CREATE TABLE backend_hosts (
    connect_id       TEXT PRIMARY KEY,
    internal_ip_addr TEXT NOT NULL,
    org_id           TEXT NOT NULL,
    user_ids         TEXT[] NOT NULL DEFAULT '{}',
    team_ids         TEXT[] NOT NULL DEFAULT '{}',
    host_user        TEXT
);

-- Teams for group-based authorization
CREATE TABLE teams (
    team_id   TEXT PRIMARY KEY,
    org_id    TEXT NOT NULL,
    user_ids  TEXT[] NOT NULL DEFAULT '{}'
);

-- Users (public_keys not used by mTLS proxy)
CREATE TABLE users (
    user_id     TEXT PRIMARY KEY,
    org_id      TEXT NOT NULL,
    public_keys BYTEA[] NOT NULL DEFAULT '{}'
);
```

### 5. Files Created

```
pkg/mtls-proxy/
├── config.go              # Configuration and validation
├── identity.go            # Certificate validation and SPIFFE parsing
├── database.go            # PostgreSQL integration and authorization
├── proxy.go               # TCP proxy server and connection handling
├── README.md              # Comprehensive documentation
├── example-start.sh       # Example startup script
└── example-client/
    └── main.go            # Test client implementation

cmd/connector/
└── main.go                # Entry point with CLI

bin/
├── connector              # Built binary (13MB)
└── mtls-test-client       # Test client binary (6.2MB)
```

### 6. Dependencies Added

```
github.com/jackc/pgx/v5         v5.7.6   # PostgreSQL driver
github.com/jackc/pgx/v5/pgxpool          # Connection pooling
github.com/sirupsen/logrus               # Logging (already present)
```

### 7. Command-Line Usage

```bash
./bin/connector \
  --listen=":8443" \
  --ca-certs="/path/to/ca1.pem,/path/to/ca2.pem" \
  --server-cert="/path/to/server.crt" \
  --server-key="/path/to/server.key" \
  --issuer="tinyscale.com" \
  --db-host="localhost" \
  --db-port=5432 \
  --db-user="tinyscale" \
  --db-password="tinyscale" \
  --db-name="tinyscale-ssh" \
  --log-level="info"
```

### 8. Key Differences from ssh-router

| Aspect | ssh-router | mTLS Proxy |
|--------|-----------|------------|
| Protocol | SSH | TCP over mTLS |
| Authentication | SSH public keys | x509 certificates |
| Identity | SSH key fingerprints | SPIFFE URIs |
| Key validation | SSH key matching | Certificate chain validation |
| Transport | SSH protocol | Raw TCP |
| Shared | Database schema, authorization logic, organization isolation |

### 9. Security Features

1. **Strong Authentication**: mTLS with certificate validation
2. **Issuer Validation**: Verifies issuer domain in SPIFFE URI
3. **Organization Isolation**: Users can only access hosts in their org
4. **Explicit Authorization**: Users must be explicitly granted access
5. **Certificate Expiry**: Validates certificates are not expired
6. **Signature Verification**: Validates certificate signatures
7. **CA Rotation Support**: Multiple CAs can be configured

### 10. Testing

**Build:**
```bash
cd /Users/jijiechen/go/src/github.com/teamycloud/tsctl
go build -o bin/connector ./cmd/connector
go build -o bin/mtls-test-client ./pkg/mtls-proxy/example-client
```

**Test Client:**
```bash
./bin/mtls-test-client \
  --proxy="localhost:8443" \
  --cert="/path/to/client.crt" \
  --key="/path/to/client.key" \
  --ca="/path/to/ca.pem" \
  --connect-id="my-host-01"
```

### 11. Future Enhancements

Potential improvements:
- [ ] Metrics and monitoring (Prometheus)
- [ ] Rate limiting per user/org
- [ ] Connection logging to database
- [ ] Certificate revocation checking (CRL/OCSP)
- [ ] Health check endpoint
- [ ] Admin API for runtime configuration
- [ ] Connection timeouts and keepalives
- [ ] TLS session resumption
- [ ] Load balancing across multiple backends

## Verification

✅ All code compiles without errors
✅ No linting errors in new code
✅ Dependencies properly added to go.mod
✅ Binaries built successfully
✅ Documentation complete
✅ Example client provided
✅ Compatible with ssh-router database schema

## Next Steps

1. Generate test certificates with SPIFFE URIs
2. Set up PostgreSQL database with test data
3. Test end-to-end connection flow
4. Deploy to test environment
5. Monitor and collect metrics
6. Plan production rollout
