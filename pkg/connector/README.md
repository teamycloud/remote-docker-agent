# mTLS Proxy - TCP Router with Certificate-Based Authentication

An mTLS proxy that provides certificate-based authentication and PostgreSQL-driven routing for TCP connections.

## Features

1. **mTLS Certificate Validation**
   - Validates client certificates against configured CA certificates
   - Supports multiple CAs for certificate rotation
   - Verifies certificate expiry and signature

2. **SNI-Based Routing**
   - Extracts connectID from SNI hostname during TLS handshake
   - Format: `<connectID>.connect.tinyscale.com`
   - Example: `host-123.connect.tinyscale.com` routes to backend host `host-123`
   - No need to send connectID in the data stream

3. **SPIFFE-Based Identity Extraction**
   - Extracts user identity from certificate SAN URI
   - Format: `spiffe://tinyscale.com/orgs/<org-id>/users/<user-id>`
   - Validates issuer domain against expected issuer

4. **PostgreSQL-Based Authorization**
   - Uses the same database schema as `ssh-router`
   - Implements identical authorization patterns
   - Supports user-based and team-based access control

5. **Dynamic Routing**
   - Routes connections to backend servers based on SNI-extracted connectID
   - Queries `backend_hosts` table for target addresses
   - Only routes when user is authorized to access the host

## Architecture

```
Client (with mTLS cert + SNI) → mTLS Proxy → PostgreSQL (authorization) → Backend Server
                                    ↓
                          SNI Extraction (connectID)
                          Certificate Validation
                          Identity Extraction
                          Authorization Check
```

## Database Schema

The proxy expects the following tables (compatible with `ssh-router`):

### `backend_hosts`
```sql
CREATE TABLE backend_hosts (
    connect_id      TEXT PRIMARY KEY,
    internal_ip_addr TEXT NOT NULL,
    org_id          TEXT NOT NULL,
    user_ids        TEXT[] NOT NULL DEFAULT '{}',
    team_ids        TEXT[] NOT NULL DEFAULT '{}',
    host_user       TEXT
);
```

### `teams`
```sql
CREATE TABLE teams (
    team_id   TEXT PRIMARY KEY,
    org_id    TEXT NOT NULL,
    user_ids  TEXT[] NOT NULL DEFAULT '{}'
);
```

### `users`
```sql
CREATE TABLE users (
    user_id     TEXT PRIMARY KEY,
    org_id      TEXT NOT NULL,
    public_keys BYTEA[] NOT NULL DEFAULT '{}'
);
```

## Authorization Logic

The proxy follows the same authorization pattern as `ssh-router`:

1. User and host must be in the same organization (`org_id` match)
2. User must be explicitly authorized via one of:
   - Direct user authorization: `user_id` in `backend_hosts.user_ids`
   - Team membership: `user_id` in a team listed in `backend_hosts.team_ids`

## Usage

### Build

```bash
go build -o bin/connector ./cmd/connector
```

### Run

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

### Command-Line Options

| Flag | Description | Default |
|------|-------------|---------|
| `--listen` | Listen address for the proxy | `:8443` |
| `--ca-certs` | Comma-separated list of CA certificate paths | (required) |
| `--server-cert` | Server certificate path | (required) |
| `--server-key` | Server private key path | (required) |
| `--issuer` | Expected issuer domain | `tinyscale.com` |
| `--db-host` | Database host | `127.0.0.1` |
| `--db-port` | Database port | `5432` |
| `--db-user` | Database user | `tinyscale` |
| `--db-password` | Database password | `tinyscale` |
| `--db-name` | Database name | `tinyscale-ssh` |
| `--log-level` | Log level (debug, info, warn, error) | `info` |

## Client Protocol

When a client connects to the proxy:

1. Client establishes mTLS connection with SNI hostname in format: `<connect_id>.connect.tinyscale.com`
   - Example: `abcdefg.connect.tinyscale.com` extracts `abcdefg` as connectID
2. Client provides valid certificate with SPIFFE URI in SAN
3. Proxy validates certificate and extracts user identity
4. Proxy extracts connectID from SNI hostname
5. Proxy queries database for authorization and routing
6. If authorized, proxy establishes backend connection and proxies data
7. If not authorized, proxy closes connection

### Example Client Code

```go
// Load client certificate
cert, err := tls.LoadX509KeyPair("client.crt", "client.key")
if err != nil {
    log.Fatal(err)
}

// Load CA certificate
caCert, err := os.ReadFile("ca.pem")
if err != nil {
    log.Fatal(err)
}
caPool := x509.NewCertPool()
caPool.AppendCertsFromPEM(caCert)

// Configure TLS with SNI
tlsConfig := &tls.Config{
    Certificates: []tls.Certificate{cert},
    RootCAs:      caPool,
    ServerName:   "connect-id-123.connect.tinyscale.com", // SNI with connectID
}

// Connect to proxy
conn, err := tls.Dial("tcp", "proxy:8443", tlsConfig)
if err != nil {
    log.Fatal(err)
}
defer conn.Close()

// Connection established, now communicate with backend
// The connectID was already sent via SNI
// ...
```

## Certificate Requirements

### Client Certificate

The client certificate must:

1. Be signed by one of the configured CA certificates
2. Be valid (not expired)
3. Contain a SPIFFE URI in the SAN field with format:
   ```
   URI:spiffe://tinyscale.com/orgs/<org-id>/users/<user-id>
   ```

### Server Certificate

Standard TLS server certificate for the proxy listener.

### CA Certificates

One or more CA certificates that sign client certificates. Multiple CAs can be configured to support CA rotation.

## Security Considerations

1. **Certificate Validation**: All client certificates are validated for expiry and signature
2. **Issuer Validation**: The issuer domain in the SPIFFE URI must match the expected issuer
3. **Authorization**: Users must be explicitly authorized in the database to access hosts
4. **Organization Isolation**: Users can only access hosts in their own organization
5. **Database Isolation**: Each organization's data is isolated by `org_id`

## Differences from ssh-router

1. **Authentication Method**: Uses mTLS client certificates instead of SSH public keys
2. **Protocol**: Raw TCP proxy instead of SSH protocol
3. **Identity Format**: SPIFFE URIs instead of SSH key fingerprints
4. **No Key Management**: Relies on certificate PKI instead of managing SSH keys

## Integration with ssh-router

The proxy can share the same PostgreSQL database as `ssh-router`:

- Uses the same `backend_hosts` table for routing
- Uses the same authorization logic (user_ids and team_ids)
- Uses the same organization isolation (`org_id`)

The main difference is that `ssh-router` validates SSH public keys while this proxy validates mTLS certificates.

## Logging

The proxy logs the following events:

- Connection accepted
- Certificate validation results
- User authentication (user_id, org_id)
- Routing decisions (connect_id, backend_addr)
- Authorization failures
- Connection errors

Log levels: `debug`, `info`, `warn`, `error`

## Monitoring

Key metrics to monitor:

- Connection count
- Authentication failures
- Authorization failures
- Backend connection failures
- Active connections
- Connection duration

## Development

### Project Structure

```
pkg/mtls-proxy/
  ├── config.go      # Configuration structures
  ├── identity.go    # Certificate validation and SPIFFE parsing
  ├── database.go    # PostgreSQL integration and authorization
  └── proxy.go       # TCP proxy server and connection handling

cmd/connector/
  └── main.go        # Entry point
```

### Adding Features

To add new features:

1. Update configuration in `config.go`
2. Implement feature logic in appropriate file
3. Update `main.go` to support new command-line flags
4. Update this README

## Testing

### Manual Testing

1. Set up PostgreSQL database with test data
2. Generate test certificates with SPIFFE URIs
3. Start the proxy
4. Connect with test client
5. Verify routing and authorization

### Database Setup

```sql
-- Create test organization
INSERT INTO backend_hosts (connect_id, internal_ip_addr, org_id, user_ids, team_ids)
VALUES ('test-host-01', '192.168.1.100:22', 'org-123', ARRAY['user-456'], ARRAY[]);

-- Create test user
INSERT INTO users (user_id, org_id, public_keys)
VALUES ('user-456', 'org-123', ARRAY[]::BYTEA[]);
```

### Certificate Generation

Generate certificates with SPIFFE URIs using tools like `spire` or `step-ca`.

## Troubleshooting

### Connection Refused

- Check that the proxy is running and listening on the correct port
- Verify firewall rules allow connections

### Certificate Validation Failed

- Verify client certificate is signed by configured CA
- Check certificate expiry date
- Verify certificate chain is complete

### Authorization Failed

- Check that user exists in database
- Verify user is in correct organization
- Confirm user is authorized (user_ids or team_ids)
- Check database connectivity

### Routing Failed

- Verify `connect_id` exists in `backend_hosts` table
- Check that `internal_ip_addr` is set and valid
- Confirm backend server is reachable

## License

Same license as the parent project.
