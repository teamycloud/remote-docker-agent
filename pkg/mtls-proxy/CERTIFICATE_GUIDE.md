# Certificate Generation Guide for mTLS Proxy

This guide shows how to generate certificates compatible with the mTLS proxy.

## Requirements

Client certificates must include a SPIFFE URI in the Subject Alternative Name (SAN) field with the format:

```
URI:spiffe://tinyscale.com/orgs/<org-id>/users/<user-id>
```

## Using OpenSSL

### 1. Generate CA Certificate

```bash
# Generate CA private key
openssl genrsa -out ca.key 4096

# Generate CA certificate
openssl req -new -x509 -days 3650 -key ca.key -out ca.pem \
  -subj "/C=US/ST=State/L=City/O=TinyScale/CN=TinyScale CA"
```

### 2. Generate Server Certificate

```bash
# Generate server private key
openssl genrsa -out server.key 2048

# Create server CSR
openssl req -new -key server.key -out server.csr \
  -subj "/C=US/ST=State/L=City/O=TinyScale/CN=proxy.tinyscale.com"

# Create server certificate config
cat > server.ext << EOF
subjectAltName = DNS:proxy.tinyscale.com,DNS:localhost,IP:127.0.0.1
extendedKeyUsage = serverAuth
EOF

# Sign server certificate
openssl x509 -req -days 365 -in server.csr \
  -CA ca.pem -CAkey ca.key -CAcreateserial \
  -out server.crt -extfile server.ext
```

### 3. Generate Client Certificate with SPIFFE URI

```bash
# Generate client private key
openssl genrsa -out client.key 2048

# Create client CSR
openssl req -new -key client.key -out client.csr \
  -subj "/C=US/ST=State/L=City/O=TinyScale/CN=user@tinyscale.com"

# Create client certificate config with SPIFFE URI
cat > client.ext << EOF
subjectAltName = URI:spiffe://tinyscale.com/orgs/org-123/users/user-456
extendedKeyUsage = clientAuth
EOF

# Sign client certificate
openssl x509 -req -days 365 -in client.csr \
  -CA ca.pem -CAkey ca.key -CAcreateserial \
  -out client.crt -extfile client.ext
```

### 4. Verify Certificate

```bash
# Verify client certificate
openssl x509 -in client.crt -text -noout | grep -A 1 "Subject Alternative Name"

# Should show:
# X509v3 Subject Alternative Name:
#     URI:spiffe://tinyscale.com/orgs/org-123/users/user-456
```

## Using step-ca (Recommended)

[step-ca](https://smallstep.com/docs/step-ca) provides better SPIFFE support.

### 1. Install step-ca

```bash
# macOS
brew install step

# Linux
wget https://github.com/smallstep/cli/releases/download/v0.25.0/step_linux_0.25.0_amd64.tar.gz
tar -xf step_linux_0.25.0_amd64.tar.gz
sudo mv step_*/bin/step /usr/local/bin/
```

### 2. Initialize CA

```bash
step ca init --name="TinyScale CA" --provisioner="admin"
```

### 3. Generate Server Certificate

```bash
step certificate create proxy.tinyscale.com server.crt server.key \
  --ca ca.pem --ca-key ca.key \
  --san proxy.tinyscale.com --san localhost \
  --kty RSA --size 2048 --not-after 8760h
```

### 4. Generate Client Certificate with SPIFFE URI

```bash
step certificate create "user-456" client.crt client.key \
  --ca ca.pem --ca-key ca.key \
  --san "spiffe://tinyscale.com/orgs/org-123/users/user-456" \
  --kty RSA --size 2048 --not-after 8760h
```

## Using SPIRE (Production)

For production environments, consider using [SPIRE](https://spiffe.io/docs/latest/spire-about/) which is designed for SPIFFE workload identity.

### 1. Install SPIRE

```bash
# Download SPIRE
wget https://github.com/spiffe/spire/releases/download/v1.8.0/spire-1.8.0-linux-amd64-musl.tar.gz
tar -xf spire-1.8.0-linux-amd64-musl.tar.gz
```

### 2. Configure SPIRE Server

```hcl
# server.conf
server {
  bind_address = "0.0.0.0"
  bind_port = "8081"
  trust_domain = "tinyscale.com"
  data_dir = "/opt/spire/data/server"
  log_level = "INFO"
}

plugins {
  DataStore "sql" {
    plugin_data {
      database_type = "postgres"
      connection_string = "postgres://user:pass@localhost/spire"
    }
  }
}
```

### 3. Configure SPIRE Agent

```hcl
# agent.conf
agent {
  trust_domain = "tinyscale.com"
  data_dir = "/opt/spire/data/agent"
}
```

### 4. Create Registration Entries

```bash
# Register a workload
spire-server entry create \
  -spiffeID spiffe://tinyscale.com/orgs/org-123/users/user-456 \
  -parentID spiffe://tinyscale.com/agent \
  -selector unix:uid:1000
```

## Certificate Validation

The proxy validates:

1. **Certificate Chain**: Must be signed by configured CA
2. **Expiry**: Certificate must not be expired
3. **SPIFFE URI**: Must contain valid SPIFFE URI in SAN
4. **Issuer Domain**: Must match configured issuer (tinyscale.com)
5. **Extended Key Usage**: Should include clientAuth

## Example Values

### Development

```
Org ID:  org-123
User ID: user-456
Issuer:  tinyscale.com
SPIFFE URI: spiffe://tinyscale.com/orgs/org-123/users/user-456
```

### Production

```
Org ID:  <UUID or customer identifier>
User ID: <UUID or email hash>
Issuer:  yourdomain.com
SPIFFE URI: spiffe://yourdomain.com/orgs/<org-id>/users/<user-id>
```

## Troubleshooting

### Certificate doesn't contain SPIFFE URI

```bash
# Verify SAN extension
openssl x509 -in client.crt -text -noout | grep -A 1 "Subject Alternative Name"
```

If missing, regenerate certificate with proper SAN configuration.

### Issuer validation fails

Check that:
1. Issuer domain in SPIFFE URI matches `--issuer` flag
2. Certificate is signed by CA in `--ca-certs` list
3. Certificate chain is complete

### Authorization fails

Check database:
```sql
-- Verify user exists
SELECT * FROM users WHERE user_id = 'user-456' AND org_id = 'org-123';

-- Verify host authorization
SELECT * FROM backend_hosts WHERE connect_id = 'my-host-01';
-- Check that user_id is in user_ids array OR
-- user is member of team in team_ids array
```

## CA Rotation

To rotate CAs:

1. Generate new CA certificate
2. Add new CA to `--ca-certs` list (comma-separated)
3. Issue new certificates with new CA
4. Remove old CA after all certificates migrated

Example:
```bash
--ca-certs="/path/to/old-ca.pem,/path/to/new-ca.pem"
```

This allows gradual migration without service disruption.
