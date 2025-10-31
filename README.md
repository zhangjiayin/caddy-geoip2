# GeoIP2

Provides middleware for resolving a users IP address against the Maxmind Geo IP Database.

Manages Downloading and Refreshing the Maxmind Database via https://github.com/maxmind/geoipupdate

## Build

```sh
xcaddy build \
  --with github.com/zhangjiayin/caddy-geoip2
```

## Caddyfile example

```
{
  order geoip2_vars first

  # Only configure databaseDirectory and editionID when autoupdate is not desired.
  #
  # To allow simultaneous lookups across multiple databases,
  # editionID can be set to a comma-separated list of database editions.
  geoip2 {
    accountId         xxxx
    databaseDirectory "/tmp/"
    licenseKey        "xxxx"
    lockFile          "/tmp/geoip2.lock"
    editionID         "GeoLite2-City"
    updateUrl         "https://updates.maxmind.com"
    updateFrequency   86400   # in seconds
  }
}

localhost {
  # - strict: Always ignore the 'X-Forwarded-For' header.
  # - wild: Trust the 'X-Forwarded-For' header if it exists.
  # - trusted_proxies: Trust the 'X-Forwarded-For' header only if trusted_proxies is valid.
  #   See: https://caddyserver.com/docs/caddyfile/options#trusted-proxies
  # - default: trusted_proxies
  geoip2_vars strict

  # Add country and state code to the header.
  header geoip-country "{geoip2.country_code}"
  header geoip-subdivision "{geoip2.subdivisions_1_iso_code}"

  # Respond to anyone in the US and Canada, but not from Ohio.
  @geofilter expression ({geoip2.country_code} == "US" || {geoip2.country_code} == "CA") && {geoip2.subdivisions_1_iso_code} != "OH"

  respond @geofilter "hello everyone except Ohioan:
    geoip2.country_code:{geoip2.country_code}
    geoip2.country_name:{geoip2.country_name}
    geoip2.city_geoname_id:{geoip2.city_geoname_id}
    geoip2.city_name:{geoip2.city_name}
    geoip2.location_latitude:{geoip2.location_latitude}
    geoip2.location_longitude:{geoip2.location_longitude}
    geoip2.location_time_zone:{geoip2.location_time_zone}"
}

```

## variables

For a complete list of available variables please check the test files in the `replacer` package.

## ref

- https://github.com/caddyserver/caddy
- https://github.com/maxmind/geoipupdate
- https://github.com/shift72/caddy-geo-ip
- https://github.com/aablinov/caddy-geoip
- https://github.com/zhangjiayin/caddy-mysql-adapter
- https://github.com/zhangjiayin/caddy-mysql-storage
