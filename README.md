# GeoIP2

Provides middleware for resolving a users IP address against the Maxmind Geo IP Database.

Manages Downloading and Refreshing the Maxmind Database via https://github.com/maxmind/geoipupdate

## Build

```
 xcaddy  \
  --with github.com/zhangjiayin/caddy-geoip2
```

## Examples

```
{
  http_port     8080
  https_port    8443
  order geoip2_vars first
  #you can only config databaseDirectory and editionID if autoupdate is not you wanted.
  geoip2 {
     accountId xxxx
     databaseDirectory "/var/lib/geoip2/data"
     licenseKey        "xxxx"
     lockFile:          ""
     editionID "GeoLite2-City"
     updateUrl   "https://updates.maxmind.com"
     updateFrequency  86400 #in seconds
   }
}

localhost:8443 {
   geoip2_vars strict #  0 false off: should turn off.   strict: not use x-forward-for  others: should general
   header geoip-country "{geoip2.country_code}"
   respond "helo geoip2:
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

- geoip2.country_code
- geoip2.country_name
- geoip2.country_eu
- geoip2.country_locales
- geoip2.country_confidence
- geoip2.country_names
- geoip2.country_geoname_id
- geoip2.continent_code
- geoip2.continent_locales
- geoip2.continent_names
- geoip2.continent_geoname_id
- geoip2.continent_name
- geoip2.city_confidence
- geoip2.city_locales
- geoip2.city_names
- geoip2.city_geoname_id
- geoip2.city_name
- geoip2.location_latitude
- geoip2.location_longitude
- geoip2.location_time_zone
- geoip2.location_accuracy_radius
- geoip2.location_average_income
- geoip2.location_metro_code
- geoip2.location_population_density
- geoip2.postal_code
- geoip2.postal_confidence
- geoip2.registeredcountry_geoname_id
- geoip2.registeredcountry_is_in_european_union
- geoip2.registeredcountry_iso_code
- geoip2.registeredcountry_names
- geoip2.registeredcountry_name
- geoip2.representedcountry_geoname_id
- geoip2.representedcountry_is_in_european_union
- geoip2.representedcountry_iso_code
- geoip2.representedcountry_names
- geoip2.representedcountry_locales
- geoip2.representedcountry_confidence
- geoip2.representedcountry_type
- geoip2.representedcountry_name
- geoip2.traits_is_anonymous_proxy
- geoip2.traits_is_anonymous_vpn
- geoip2.traits_is_satellite_provider
- geoip2.traits_autonomous_system_number
- geoip2.traits_autonomous_system_organization
- geoip2.traits_autonomous_system_organization
- geoip2.traits_autonomous_system_organization
- geoip2.traits_connection_type
- geoip2.traits_domain
- geoip2.traits_is_hosting_provider
- geoip2.traits_is_legitimate_proxy
- geoip2.traits_is_public_proxy
- geoip2.traits_is_residential_proxy
- geoip2.traits_is_tor_exit_node
- geoip2.traits_isp
- geoip2.traits_mobile_country_code
- geoip2.traits_mobile_network_code
- geoip2.traits_network
- geoip2.traits_organization
- geoip2.traits_user_type
- geoip2.traits_userCount
- geoip2.traits_static_ip_score

## ref

- https://github.com/caddyserver/caddy
- https://github.com/maxmind/geoipupdate
- https://github.com/shift72/caddy-geo-ip
- https://github.com/aablinov/caddy-geoip
- https://github.com/zhangjiayin/caddy-mysql-adapter
- https://github.com/zhangjiayin/caddy-mysql-storage
