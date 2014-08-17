ipLocator
=========

ipLocator - a basic Geo-Ip Server made with Go

===

Dependencies:

1. nosql - database boltdb

go get github.com/boltdb/bolt

===

2. bloomfilter

go get github.com/AndreasBriese/bbloom

===

## Usage

Configure ipLocator with command line options (default values shown)

  -download_DB=false: Reload database from maxmind.com and Restore database from GeoLite-Data
  
  -ip="": enter a csv-list of IP
  
  -json=false: return JSON
  
  -new_DB=false: Restore database from maxmind.com GeoLite-Data
  
  -server=false: run server at localhost:9000
  
