# GeoIP

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
   respond "Hello, world {geoip2.country_code}!"  #geoip2.country_code  geoip2.country_name geoip2.country_eu geoip2.city_name geoip2.latitude geoip2.longitude geoip2.time_zone
}

```

## variables
 - geoip2.country_code  
 - geoip2.country_name 
 - geoip2.country_eu 
 - geoip2.city_name 
 - geoip2.latitude 
 - geoip2.longitude 
 - geoip2.time_zone


## Warning 
 it is not stable now.


## ref

 - https://github.com/caddyserver/caddy
 - https://github.com/maxmind/geoipupdate
 - https://github.com/shift72/caddy-geo-ip
 - https://github.com/aablinov/caddy-geoip
 - https://github.com/zhangjiayin/caddy-mysql-adapter
 - https://github.com/zhangjiayin/caddy-mysql-storage
