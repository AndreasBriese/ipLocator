ipLocator
=========

ipLocator - a fast basic Geo-Ip Server made with Go

===

#### Dependencies

(1) pure go key/value store <b>boltdb</b> (https://github.com/boltdb/bolt)

    go get github.com/boltdb/bolt


(2) bloomfilter

    go get github.com/AndreasBriese/bbloom

===

#### Usage

Configure ipLocator with command line options (default values shown)

 -download_DB=false: Reload database from maxmind.com and Restore database from GeoLite-Data
  
 -ip="": enter a csv-list of IP
  
 -json=false: return JSON
  
 -new_DB=false: Restore database from maxmind.com GeoLite-Data
  
 -server=false: run server at localhost:9000
  

#### Quickstart:

#####Linux/MacOsX

    go run ipLocator.go -download_DB=true -server=true
    
1. downloads the maxmind.com GeoLite2 - CSV .zip database folder
2. unzips it  
3. loads csv-data into programs database ./iplocs.bdb (~ 500 MB)
4. starts server at localhost:9000


#####Windows 

Download the zipped database folder from maxmind.com manually, unzip it (however) and copy GeoLite2-City-Blocks.csv and GeoLite2-City-Locations.csv into the folder containing ipLocator.go

Run ipLocator with

    go run ipLocator.go -new_DB=true -server=true

3. loads csv-data into programs database ./iplocs.bdb (~ 500 MB)
4. starts server at localhost:9000
  


===

As of 2014-08-17 a demo server is running at https://oo.bootes.uberspace.de 

