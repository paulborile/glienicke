# TLS Certificates for Nostr Relay v0.13.0

This document explains how to generate and use TLS certificates for secure WebSocket (WSS) connections with the Nostr relay.

**New in v0.13.0**: Complete WSS/TLS support with certificate management tools and comprehensive testing infrastructure.

## Quick Start

For development, the easiest approach is to use `mkcert`:

```bash
# Install mkcert (Linux)
curl -JLO "https://dl.filippo.io/mkcert/latest?for=linux/amd64"
chmod +x mkcert-v*-linux-amd64
sudo mv mkcert-v*-linux-amd64 /usr/local/bin/mkcert

# Create local CA and install it
mkcert -install

# Generate certificate for localhost
mkcert paul-zbook 127.0.0.1 ::1

# Start relay with TLS
./relay -cert localhost+2.pem -key localhost+2-key.pem -addr :8443
```

## Certificate Generation Methods

### Method 1: mkcert (Recommended for Development)

`mkcert` creates locally-trusted certificates without browser warnings.

```bash
# Install mkcert
curl -JLO "https://dl.filippo.io/mkcert/latest?for=linux/amd64"
chmod +x mkcert-v*-linux-amd64
sudo mv mkcert-v*-linux-amd64 /usr/local/bin/mkcert

# Create and install local CA
mkcert -install

# Generate certificate for your domains
mkcert relay.paulstephenborile.com localhost 127.0.0.1 ::1

# This creates in current directory:
# - relay.paulstephenborile.com+3.pem (certificate)
# - relay.paulstephenborile.com+3-key.pem (private key)
# 
# Move to resources folder:
# mv relay.paulstephenborile.com+3.pem resources/
# mv relay.paulstephenborile.com+3-key.pem resources/
```

**Files created:**
- `localhost+2.pem` - TLS certificate
- `localhost+2-key.pem` - Private key
- `~/.local/share/mkcert/rootCA.pem` - CA certificate (for import into clients)

### Method 2: OpenSSL (Universal)

Generate self-signed certificates with proper Subject Alternative Names (SANs).

```bash
# Create config file
cat > cert-config.cnf << 'EOF'
[req]
default_bits = 2048
prompt = no
default_md = sha256
distinguished_name = dn
x509_extensions = v3_req

[dn]
C = US
ST = State
L = City
O = Nostr Relay
CN = localhost

[v3_req]
subjectAltName = @alt_names
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth

[alt_names]
DNS.1 = localhost
DNS.2 = *.localhost
IP.1 = 127.0.0.1
IP.2 = ::1
EOF

# Generate certificate
openssl req -x509 -newkey rsa:2048 -keyout resources/relay-key.pem -out resources/relay-cert.pem -days 365 -nodes -config resources/cert-config.cnf
```

### Method 3: Let's Encrypt (Production)

For public domains, use Let's Encrypt for free, trusted certificates.

```bash
# Install certbot
sudo apt install certbot

# Get certificate for your domain
sudo certbot certonly --standalone -d relay.paulstephenborile.com

# Certificate files location:
# /etc/letsencrypt/live/relay.paulstephenborile.com/fullchain.pem
# /etc/letsencrypt/live/relay.paulstephenborile.com/privkey.pem
```

### Method 4: Simple OpenSSL (Quick Development)

```bash
# Basic self-signed certificate
openssl req -x509 -newkey rsa:2048 -keyout resources/relay-key.pem -out resources/relay-cert.pem -days 365 -nodes -subj "/C=US/ST=State/L=City/O=Organization/CN=localhost"
```

## Using Certificates with the Relay

### Command Line Usage

```bash
# Start relay with TLS
./relay -cert resources/relay-cert.pem -key resources/relay-key.pem -addr :8443

# With Let's Encrypt certificates for production domain
./relay -cert /etc/letsencrypt/live/relay.paulstephenborile.com/fullchain.pem -key /etc/letsencrypt/live/relay.paulstephenborile.com/privkey.pem -addr :443

# With mkcert certificates (move to resources first)
./relay -cert resources/relay.paulstephenborile.com+3.pem -key resources/relay.paulstephenborile.com+3-key.pem -addr :8443
```

### Certificate Files

- **Certificate file**: Public certificate (`.pem`, `.crt`, `.cer`) - store in `resources/`
- **Key file**: Private key (`.pem`, `.key`) - store in `resources/`
- **Chain file**: Certificate chain (for Let's Encrypt: `fullchain.pem`)

## Client Trust Configuration

### Self-Signed Certificates

For self-signed certificates (OpenSSL method), clients need to trust the certificate:

#### Option 1: Import CA Certificate

```bash
# Export the certificate for clients
openssl x509 -in resources/relay-cert.pem -out resources/relay-cert.crt -outform DER
```

Then import `relay-cert.crt` into:
- **System trust store**: 
  ```bash
  sudo cp relay-cert.crt /usr/local/share/ca-certificates/
  sudo update-ca-certificates
  ```
- **Firefox**: Settings → Privacy & Security → Certificates → View Certificates → Authorities → Import
- **Chrome/Chromium**: Settings → Privacy and security → Manage certificates → Authorities → Import

#### Option 2: Client-Side Configuration

Most Nostr clients have options to:
- Skip certificate verification (testing only)
- Accept specific self-signed certificates
- Use custom CA certificates

#### Option 3: mkcert (Recommended)

mkcert certificates are automatically trusted if you run `mkcert -install`. If not trusted:

```bash
# Copy the CA certificate
cp ~/.local/share/mkcert/rootCA.pem resources/mkcert-ca.crt
# Then install to system:
sudo cp resources/mkcert-ca.crt /usr/local/share/ca-certificates/mkcert-ca.crt
sudo update-ca-certificates
```

### Production Domains

For public domains, use Let's Encrypt. These certificates are trusted by all clients automatically.

## Testing the Certificate

```bash
# Test TLS connection
openssl s_client -connect localhost:8443 -showcerts

# Check certificate details
openssl x509 -in resources/relay-cert.pem -text -noout

# Test with curl
curl -k -v https://localhost:8443/
```

## Common Issues and Solutions

### "remote error: tls: unknown certificate authority"

**Cause**: Client doesn't trust the self-signed certificate.

**Solutions**:
1. Use mkcert: `mkcert -install` then `mkcert localhost`
2. Import the certificate into the client's trust store
3. Use a domain with Let's Encrypt certificate
4. Configure client to skip certificate verification (testing only)

### "certificate has expired"

**Solution**: Generate a new certificate or renew Let's Encrypt certificate.

### "certificate doesn't contain SANs"

**Cause**: Old certificate format without Subject Alternative Names.

**Solution**: Use the OpenSSL config method or mkcert which includes proper SANs.

## Security Considerations

- **Development**: Use mkcert or self-signed certificates
- **Staging**: Use Let's Encrypt staging environment
- **Production**: Use Let's Encrypt or commercial certificates
- **Never**: Use development certificates in production

## Certificate Renewal

### Self-Signed Certificates
- Manually regenerate before expiry
- Typically valid for 1 year

### Let's Encrypt
```bash
# Renew all certificates
sudo certbot renew

# Test renewal process
sudo certbot renew --dry-run
```

### mkcert
- Valid for 3 years
- Regenerate with `mkcert localhost 127.0.0.1 ::1`

## File Naming Conventions

Recommended file naming in `resources/` folder:
- `resources/relay-cert.pem` - Certificate file
- `resources/relay-key.pem` - Private key file  
- `resources/localhost+2.pem` - mkcert certificate
- `resources/localhost+2-key.pem` - mkcert private key
- `resources/relay-cert.crt` - DER format certificate (for client import)
- `resources/cert-config.cnf` - OpenSSL configuration file

Let's Encrypt certificates remain in system location:
- `/etc/letsencrypt/live/relay.paulstephenborile.com/fullchain.pem` - Certificate chain
- `/etc/letsencrypt/live/relay.paulstephenborile.com/privkey.pem` - Private key

## Quick Reference

```bash
# Production (mkcert - recommended for development)
mkcert -install && mkcert relay.paulstephenborile.com localhost 127.0.0.1 ::1
mv relay.paulstephenborile.com+3.pem resources/ && mv relay.paulstephenborile.com+3-key.pem resources/
./relay -cert resources/relay.paulstephenborile.com+3.pem -key resources/relay.paulstephenborile.com+3-key.pem -addr :8443

# Development (OpenSSL with production domain)
openssl req -x509 -newkey rsa:2048 -keyout resources/relay-key.pem -out resources/relay-cert.pem -days 365 -nodes -config resources/cert-config.cnf
./relay -cert resources/relay-cert.pem -key resources/relay-key.pem -addr :8443

# Production (Let's Encrypt)
sudo certbot certonly --standalone -d your-domain.com
./relay -cert /etc/letsencrypt/live/your-domain.com/fullchain.pem -key /etc/letsencrypt/live/your-domain.com/privkey.pem -addr :443
```

## Connecting to WSS Relay

Once the relay is running with TLS, connect using:
- **Local development**: `wss://localhost:8443`
- **Production domain**: `wss://relay.paulstephenborile.com`
- **With IP**: `wss://127.0.0.1:8443` (may need to accept certificate warning)

### Example Connection URLs
- **Development**: `wss://localhost:8443`
- **Production**: `wss://relay.paulstephenborile.com` (requires port 443 and Let's Encrypt cert)
- **Production with custom port**: `wss://relay.paulstephenborile.com:8443`
