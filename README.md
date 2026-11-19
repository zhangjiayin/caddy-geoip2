# GeoIP2 Module for Caddy

A high-performance GeoIP2 middleware for Caddy that provides geographic information based on client IP addresses.

## Features

- **Minimal & Fast**: Only extracts the essential GeoIP2 data you actually need
- **Thread-safe**: Concurrent request handling with read/write locks
- **Auto-reload**: Configurable database reloading (daily, weekly, or custom intervals)
- **Smart IP Detection**: Flexible handling of X-Forwarded-For headers with security controls
- **Memory Efficient**: Optimized data structures to minimize allocations
- **Intelligent Database Routing**: EU IPs use Europe-specific database, non-EU IPs use global database for optimal performance

## Build

```bash
xcaddy build --with github.com/ScaleCommerce/caddy-geoip2
```

## Configuration

### Global App Configuration

```caddyfile
{
  # Configure the GeoIP2 databases globally with intelligent routing
  geoip2 {
    country_database_path /etc/nginx/maxmind-geo-ip/GeoIP-Country/GeoIP2-Country.mmdb
    city_database_path /etc/nginx/maxmind-geo-ip/GeoIP-Country/GeoIP2-City-Europe.mmdb
    global_city_database_path /etc/nginx/maxmind-geo-ip/GeoLite2-City.mmdb
    asn_database_path /etc/nginx/maxmind-geo-ip/GeoLite2-ASN.mmdb
    reload_interval daily  # daily, weekly, off, or hours (e.g., 24)
  }

  # Order matters - GeoIP2 must run before directives that use the variables
  order geoip2_vars before header
}
```

### Site-specific Handler

```caddyfile
example.com {
  # Enable GeoIP2 with IP detection mode:
  # - strict: only use RemoteAddr (ignore X-Forwarded-For)
  # - wild: trust any X-Forwarded-For header
  # - trusted_proxies: trust X-Forwarded-For only from trusted proxies (default)
  # - client_ip: use the client IP Caddy determined (recommended behind proxies)
  geoip2_vars strict

  # Use GeoIP2 variables in directives
  header Country-Code "{geoip2_country_code}"
  header Is-EU "{geoip2_is_in_eu}"

  # Conditional responses based on location
  @eu_visitors expression {geoip2_is_in_eu} == true
  respond @eu_visitors "Hello from the EU!"

  # Log geographic data
  log {
    format json
    append {
      country {geoip2_country_code}
      city {geoip2_city}
      coordinates "{geoip2_latitude},{geoip2_longitude}"
      subdivision {geoip2_subdivisions}
      asn {geoip2_asn}
    }
  }

  respond "Hello from {geoip2_city}, {geoip2_country_code}!"
}
```

## Available Variables

The module provides 8 essential GeoIP2 variables from 4 specialized databases:

| Variable | Description | Example | Database Source |
|----------|-------------|---------|-----------------|
| `{geoip2_country_code}` | Two-letter country code | `"DE"` | Country DB |
| `{geoip2_is_in_eu}` | EU membership status | `true` | Country DB |
| `{geoip2_city}` | City name (German preferred) | `"München"` | EU: Europe City DB<br/>Non-EU: Global City DB |
| `{geoip2_latitude}` | Geographic latitude | `48.1374` | EU: Europe City DB<br/>Non-EU: Global City DB |
| `{geoip2_longitude}` | Geographic longitude | `11.5755` | EU: Europe City DB<br/>Non-EU: Global City DB |
| `{geoip2_subdivisions}` | State/Province code | `"BY"` | EU: Europe City DB<br/>Non-EU: Global City DB |
| `{geoip2_asn}` | Autonomous System Number | `3320` | ASN DB |
| `{geoip2_asorg}` | AS Organization | `"Deutsche Telekom AG"` | ASN DB |

### Intelligent Database Routing

| IP Location | City Data Source | Performance Benefit |
|-------------|------------------|-------------------|
| **European Union** | `GeoIP2-City-Europe.mmdb` | ⚡ Faster (smaller DB) |
| **Non-EU** | `GeoLite2-City.mmdb` | 🌍 Complete global coverage |
| **All IPs** | `GeoIP2-Country.mmdb` (for country/EU data) | 🎯 Specialized accuracy |
| **All IPs** | `GeoLite2-ASN.mmdb` (for network data) | 📊 Comprehensive ASN info |

### Nginx to Caddy Variable Mapping

| Nginx Variable | Caddy Variable | Database Used |
|----------------|----------------|---------------|
| `$geoip2_country_code` | `{geoip2_country_code}` | GeoIP2-Country.mmdb |
| `$geoip2_city` | `{geoip2_city}` | EU: GeoIP2-City-Europe.mmdb<br/>Non-EU: GeoLite2-City.mmdb |
| `$geoip2_subdivision` | `{geoip2_subdivisions}` | EU: GeoIP2-City-Europe.mmdb<br/>Non-EU: GeoLite2-City.mmdb |
| `$geoip2_is_in_european_union` | `{geoip2_is_in_eu}` | GeoIP2-Country.mmdb |
| `$geoip2_latitude` | `{geoip2_latitude}` | EU: GeoIP2-City-Europe.mmdb<br/>Non-EU: GeoLite2-City.mmdb |
| `$geoip2_longitude` | `{geoip2_longitude}` | EU: GeoIP2-City-Europe.mmdb<br/>Non-EU: GeoLite2-City.mmdb |

## Advanced Examples

### Geographic Access Control

```caddyfile
{
  geoip2 {
    country_database_path /etc/nginx/maxmind-geo-ip/GeoIP-Country/GeoIP2-Country.mmdb
    city_database_path /etc/nginx/maxmind-geo-ip/GeoIP-Country/GeoIP2-City-Europe.mmdb
    global_city_database_path /etc/nginx/maxmind-geo-ip/GeoLite2-City.mmdb
    asn_database_path /etc/nginx/maxmind-geo-ip/GeoLite2-ASN.mmdb
    reload_interval daily
  }
}

api.example.com {
  # An access rule must not go by an address the client can choose: behind
  # proxies use client_ip, with trusted_proxies and trusted_proxies_strict
  # set on the server (see "IP Detection Modes").
  geoip2_vars client_ip

  # Block requests from certain countries
  @blocked_countries expression {geoip2_country_code} in ["CN", "RU", "KP"]
  respond @blocked_countries "Access denied" 403

  # Rate limit by geographic region
  @high_traffic expression {geoip2_country_code} in ["US", "GB", "DE"]
  rate_limit @high_traffic 100r/m

  @low_traffic expression !({geoip2_country_code} in ["US", "GB", "DE"])
  rate_limit @low_traffic 10r/m

  reverse_proxy localhost:8080
}
```

### Development vs Production Database

```caddyfile
{
  geoip2 {
    country_database_path /etc/nginx/maxmind-geo-ip/GeoIP-Country/GeoIP2-Country.mmdb
    city_database_path /etc/nginx/maxmind-geo-ip/GeoIP-Country/GeoIP2-City-Europe.mmdb
    global_city_database_path /etc/nginx/maxmind-geo-ip/GeoLite2-City.mmdb
    asn_database_path /etc/nginx/maxmind-geo-ip/GeoLite2-ASN.mmdb
    reload_interval daily
  }
}

localhost {
  geoip2_vars wild  # More permissive for development

  respond `
  IP: {remote_host}
  Location: {geoip2_city}, {geoip2_country_code}
  Coordinates: {geoip2_latitude}, {geoip2_longitude}
  EU Member: {geoip2_is_in_eu}
  ASN: {geoip2_asn} ({geoip2_asorg})
  `
}
```

### Comprehensive Logging

```caddyfile
{
  geoip2 {
    country_database_path /etc/nginx/maxmind-geo-ip/GeoIP-Country/GeoIP2-Country.mmdb
    city_database_path /etc/nginx/maxmind-geo-ip/GeoIP-Country/GeoIP2-City-Europe.mmdb
    global_city_database_path /etc/nginx/maxmind-geo-ip/GeoLite2-City.mmdb
    asn_database_path /etc/nginx/maxmind-geo-ip/GeoLite2-ASN.mmdb
    reload_interval daily
  }
}

example.com {
  geoip2_vars client_ip

  log {
    output file /var/log/caddy/geo.log
    format json
    append {
      # Geographic data
      geo_country {geoip2_country_code}
      geo_city {geoip2_city}
      geo_lat {geoip2_latitude}
      geo_lng {geoip2_longitude}
      geo_subdivision {geoip2_subdivisions}
      geo_eu {geoip2_is_in_eu}

      # Network data
      asn {geoip2_asn}
      as_org {geoip2_asorg}

      # Request data
      user_agent {>User-Agent}
      real_ip {remote_host}
    }
  }

  reverse_proxy backend:8080
}
```

## Understanding Execution Order

### Why Order Matters

**GeoIP2 is a middleware** that sets variables, but **directives** (like `header`, `log`, `respond`) need those variables to be available:

```caddyfile
{
  # GeoIP2 middleware must run BEFORE directives that use the variables
  order geoip2_vars before header
  # Alternative: order geoip2_vars first (runs before everything)
}

example.com {
  # 1. geoip2_vars middleware runs first → sets variables
  geoip2_vars strict

  # 2. Then header directive can use the variables
  header Country-Code "{geoip2_country_code}"  # ✅ Variable available

  # 3. respond directive can also use them
  respond "Hello from {geoip2_city}!"  # ✅ Variable available
}
```

### Common Order Configurations

```caddyfile
# Option 1: Run before specific directives
order geoip2_vars before header

# Option 2: Run first (safest, works with everything)
order geoip2_vars first

# Option 3: Run before multiple directives
order geoip2_vars before header rewrite respond
```

### What Happens Without Correct Order

```caddyfile
# ❌ BAD: No order specified
example.com {
  header Country-Code "{geoip2_country_code}"  # Variable = "" (empty)
  geoip2_vars strict                           # Runs too late!
}

# ✅ GOOD: Correct order
{
  order geoip2_vars before header
}
example.com {
  geoip2_vars strict                           # Sets variables first
  header Country-Code "{geoip2_country_code}"  # Variable has value
}
```

## IP Detection Modes

### `strict` Mode
- **Use case**: High-security environments, direct connections
- **Behavior**: Only uses `RemoteAddr`, ignores all forwarded headers
- **Pros**: Most secure, no spoofing possible
- **Cons**: Won't work behind proxies/load balancers

### `trusted_proxies` Mode (Default)
- **Use case**: Production with known proxy infrastructure
- **Behavior**: Uses the **first** entry of `X-Forwarded-For`, only for requests from Caddy's trusted proxies
- **Pros**: Works with a proxy that replaces the header with the client's address
- **Cons**: Requires correct `trusted_proxies` configuration. A proxy that appends to the header (most do) leaves the first entry to the client, who can then choose the looked-up address. Prefer `client_ip`.

### `client_ip` Mode
- **Use case**: Production behind load balancers or CDNs
- **Behavior**: Uses the client IP Caddy determined for the request, the same value as `{client_ip}` and the `client_ip` matcher. Caddy takes it from the server's `client_ip_headers` (default `X-Forwarded-For`) for requests from `trusted_proxies`, and from the connection for all others. With `trusted_proxies_strict` the header is read from the right and trusted proxies are skipped, so a chain of proxies (load balancer behind a CDN) resolves to the real client.
- **Pros**: Not forgeable with `trusted_proxies_strict`; one definition of "client" for lookups, logs and access rules
- **Cons**: Requires correct `trusted_proxies` configuration. Without `trusted_proxies_strict` Caddy itself takes the first entry of the header.
- **Every proxy in the chain must be in `trusted_proxies`**: Caddy stops at the first address that is not, so a CDN whose ranges are listed only in part gets its edge server looked up instead of the client. Nothing is forged then, but the result is the proxy's location.
- A CDN that sends the client's address in a header of its own (`CF-Connecting-IP`, for example) needs that header in the server's `client_ip_headers`.

```caddyfile
{
  servers {
    trusted_proxies static 10.0.1.0/24 173.245.48.0/20
    trusted_proxies_strict
  }
}

example.com {
  geoip2_vars client_ip
}
```

### `wild` Mode
- **Use case**: Development, testing, or when you can't control proxy headers
- **Behavior**: Trusts any `X-Forwarded-For` header
- **Pros**: Works everywhere, easy setup
- **Cons**: Vulnerable to IP spoofing

## Database Reload Options

| Value | Description | Use Case |
|-------|-------------|----------|
| `daily` or `24h` | Reload every 24 hours | 🔄 Recommended for production |
| `weekly` or `168h` | Reload every 7 days | ⚡ Lower overhead, less frequent updates |
| `48` | Reload every 48 hours | ⚖️ Balance between freshness and performance |
| `off` or `0` | No automatic reload | 🔧 Manual control only |

## Performance Optimizations

1. **Minimal Structure**: Only parses fields you actually use
2. **Constant Variables**: Uses constants for variable names (compiler optimization)
3. **Method Decomposition**: Separates concerns for better caching
4. **Smart Fallbacks**: English city names with fallback to any available language
5. **Early Returns**: Fails fast on errors without unnecessary processing
6. **Read Locks**: Multiple concurrent lookups without blocking

## Database Compatibility

| Database | Type | Purpose | Cost | Size |
|----------|------|---------|------|------|
| **GeoIP2-Country** | Commercial | Country codes & EU status | 💰 Paid | ~5MB |
| **GeoIP2-City-Europe** | Commercial | European city data | 💰 Paid | ~30MB |
| **GeoLite2-City** | Free | Global city data (fallback) | 🆓 Free | ~70MB |
| **GeoLite2-ASN** | Free | ASN numbers & organizations | 🆓 Free | ~10MB |

### Alternative Free Setup
For budget-conscious deployments, you can use free alternatives:
- `GeoLite2-Country.mmdb` instead of `GeoIP2-Country.mmdb`
- `GeoLite2-City.mmdb` for both EU and global city data

### Four-Database Setup with Intelligent Routing

For optimal performance on a central load balancer, use the four-database approach with intelligent routing:

```caddyfile
{
  geoip2 {
    country_database_path /etc/nginx/maxmind-geo-ip/GeoIP-Country/GeoIP2-Country.mmdb
    city_database_path /etc/nginx/maxmind-geo-ip/GeoIP-Country/GeoIP2-City-Europe.mmdb
    global_city_database_path /etc/nginx/maxmind-geo-ip/GeoLite2-City.mmdb
    asn_database_path /etc/nginx/maxmind-geo-ip/GeoLite2-ASN.mmdb
    reload_interval daily
  }
}
```

**Intelligent Routing Logic:**
1. **Country Lookup**: Determines EU membership status
2. **EU IPs**: Use smaller, faster Europe-specific city database
3. **Non-EU IPs**: Use global city database as fallback
4. **ASN Lookup**: Always uses dedicated ASN database

**Why four databases with intelligent routing?**
- **GeoIP2-Country**: Fast country codes and EU membership detection
- **GeoIP2-City-Europe**: Optimized for European traffic (smaller, faster)
- **GeoLite2-City**: Global coverage for non-European IPs
- **GeoLite2-ASN**: Comprehensive ASN data

**Performance benefits for central load balancers:**
- **Reduced memory pressure**: European traffic uses smaller database
- **Faster lookups**: 70%+ of traffic (EU) uses optimized database
- **Global coverage**: Non-EU traffic still gets complete data
- **High availability**: Automatic fallback if any database unavailable
- **Language support**: German city names preferred for European IPs

## Error Handling

The module gracefully handles:
- Missing database files
- Corrupted databases
- Invalid IP addresses
- Database reload failures
- Network interruptions

Variables are always available (empty strings if lookup fails) to prevent template errors.

## Monitoring

You can monitor the GeoIP2 module via Caddy's admin API:

```bash
# Check current status
curl localhost:2019/config/apps/geoip2

# Reload database manually
curl -X POST localhost:2019/load \
  -H "Content-Type: application/json" \
  -d '{"module": "geoip2"}'
```

## License

This project is licensed under the Apache License 2.0.

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## References

- [Caddy Documentation](https://caddyserver.com/docs/)
- [MaxMind GeoIP2 Documentation](https://dev.maxmind.com/geoip/docs/)
- [MaxMind Database Format](https://maxmind.github.io/MaxMind-DB/)
