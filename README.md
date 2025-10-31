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

  # Only configure databaseDirectory and editionIDs when autoupdate is not desired.
  #
  # To allow simultaneous lookups across multiple databases,
  # editionIDs can be set to a comma-separated list of database editions.
  geoip2 {
    accountId         xxxx
    databaseDirectory "/tmp/"
    licenseKey        "xxxx"
    lockFile          "/tmp/geoip2.lock"
    editionIDs        "GeoLite2-City,GeoLite2-ASN"
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
For a complete list of available variables please check the test files in the
`replacer` package. 
### Anonymous
| Variable | Description |
| --- | --- |
| `geoip2.is_anonymous` | Whether the IP address is anonymous. |
| `geoip2.is_anonymous_vpn` | Whether the IP address is an anonymous VPN. |
| `geoip2.is_hosting_provider` | Whether the IP address is a hosting provider. |
| `geoip2.is_public_proxy` | Whether the IP address is a public proxy. |
| `geoip2.is_residential_proxy` | Whether the IP address is a residential proxy. |
| `geoip2.is_tor_exit_node` | Whether the IP address is a Tor exit node. |

### Connection-Type
| Variable | Description |
| --- | --- |
| `geoip2.connection_type` | The connection type of the IP address. |

### Domain
| Variable | Description |
| --- | --- |
| `geoip2.domain` | The domain of the IP address. |

### Enterprise
| Variable | Description |
| --- | --- |
| `geoip2.country_code` | The country code of the IP address. |
| `geoip2.country_confidence` | The confidence of the country lookup. |
| `geoip2.country_eu` | Whether the country is in the European Union. |
| `geoip2.country_geoname_id` | The GeoName ID of the country. |
| `geoip2.country_name` | The name of the country. |
| `geoip2.country_names` | The names of the country in different languages. |
| `geoip2.country_names_*` | The name of the country in a specific language. |
| `geoip2.continent_code` | The continent code of the IP address. |
| `geoip2.continent_geoname_id` | The GeoName ID of the continent. |
| `geoip2.continent_name` | The name of the continent. |
| `geoip2.continent_names` | The names of the continent in different languages. |
| `geoip2.continent_names_*` | The name of the continent in a specific language. |
| `geoip2.city_confidence` | The confidence of the city lookup. |
| `geoip2.city_geoname_id` | The GeoName ID of the city. |
| `geoip2.city_name` | The name of the city. |
| `geoip2.city_names` | The names of the city in different languages. |
| `geoip2.city_names_*` | The name of the city in a specific language. |
| `geoip2.location_latitude` | The latitude of the IP address. |
| `geoip2.location_longitude` | The longitude of the IP address. |
| `geoip2.location_time_zone` | The time zone of the IP address. |
| `geoip2.location_accuracy_radius` | The accuracy radius of the location. |
| `geoip2.location_metro_code` | The metro code of the location. |
| `geoip2.postal_code` | The postal code of the IP address. |
| `geoip2.postal_confidence` | The confidence of the postal code lookup. |
| `geoip2.registeredcountry_geoname_id` | The GeoName ID of the registered country. |
| `geoip2.registeredcountry_is_in_european_union` | Whether the registered country is in the European Union. |
| `geoip2.registeredcountry_iso_code` | The ISO code of the registered country. |
| `geoip2.registeredcountry_name` | The name of the registered country. |
| `geoip2.registeredcountry_names` | The names of the registered country in different languages. |
| `geoip2.registeredcountry_names_*` | The name of the registered country in a specific language. |
| `geoip2.representedcountry_geoname_id` | The GeoName ID of the represented country. |
| `geoip2.representedcountry_is_in_european_union` | Whether the represented country is in the European Union. |
| `geoip2.representedcountry_iso_code` | The ISO code of the represented country. |
| `geoip2.representedcountry_name` | The name of the represented country. |
| `geoip2.representedcountry_names` | The names of the represented country in different languages. |
| `geoip2.representedcountry_names_*` | The name of the represented country in a specific language. |
| `geoip2.representedcountry_type` | The type of the represented country. |
| `geoip2.subdivisions` | The subdivisions of the IP address. |
| `geoip2.subdivisions_*_confidence` | The confidence of the subdivision lookup. |
| `geoip2.subdivisions_*_geoname_id` | The GeoName ID of the subdivision. |
| `geoip2.subdivisions_*_iso_code` | The ISO code of the subdivision. |
| `geoip2.subdivisions_*_name` | The name of the subdivision. |
| `geoip2.subdivisions_*_names` | The names of the subdivision in different languages. |
| `geoip2.subdivisions_*_names_*` | The name of the subdivision in a specific language. |
| `geoip2.traits_autonomous_system_number` | The autonomous system number of the IP address. |
| `geoip2.traits_autonomous_system_organization` | The autonomous system organization of the IP address. |
| `geoip2.traits_connection_type` | The connection type of the IP address. |
| `geoip2.traits_domain` | The domain of the IP address. |
| `geoip2.traits_is_anonymous_proxy` | Whether the IP address is an anonymous proxy. |
| `geoip2.traits_is_anycast` | Whether the IP address is an anycast address. |
| `geoip2.traits_is_legitimate_proxy` | Whether the IP address is a legitimate proxy. |
| `geoip2.traits_is_satellite_provider` | Whether the IP address is a satellite provider. |
| `geoip2.traits_isp` | The ISP of the IP address. |
| `geoip2.traits_mobile_country_code` | The mobile country code of the IP address. |
| `geoip2.traits_mobile_network_code` | The mobile network code of the IP address. |
| `geoip2.traits_organization` | The organization of the IP address. |
| `geoip2.traits_static_ip_score` | The static IP score of the IP address. |
| `geoip2.traits_user_type` | The user type of the IP address. |

### ISP
| Variable | Description |
| --- | --- |
| `geoip2.autonomous_system_number` | The autonomous system number of the IP address. |
| `geoip2.autonomous_system_organization` | The autonomous system organization of the IP address. |
| `geoip2.isp` | The ISP of the IP address. |
| `geoip2.mobile_country_code` | The mobile country code of the IP address. |
| `geoip2.mobile_network_code` | The mobile network code of the IP address. |
| `geoip2.organization` | The organization of the IP address. |

### Replacer
| Variable | Description |
| --- | --- |
| `geoip2.ip_address` | The IP address of the user. |

## ref

- https://github.com/caddyserver/caddy
- https://github.com/maxmind/geoipupdate
- https://github.com/shift72/caddy-geo-ip
- https://github.com/aablinov/caddy-geoip
- https://github.com/zhangjiayin/caddy-mysql-adapter
- https://github.com/zhangjiayin/caddy-mysql-storage
