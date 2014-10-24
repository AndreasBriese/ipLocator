// The MIT License (MIT)

// Copyright (c) 2014 Andreas Briese, eduToolbox@Bri-C GmbH, Sarstedt GERMANY

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

//

package main

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"flag"
	"fmt"
	"github.com/AndreasBriese/bbloom"
	"github.com/boltdb/bolt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var infoHTML = `
<!DOCTYPE html>
<head>
	<meta charset="utf-8" />
	<title>ipLocator - get city locations from ip-numbers</title>
	<style>
		p {
			margin-left: 10%;
			margin-right: 5%;
		}
		#api{
			margin-left: 5%;
			margin-right: 5%;
		}
	</style>
	<script>
	  function getLoc(){
	    ip = document.getElementById("ip").value;
	    if(ip){
	      try{
			var xmlhttp = new XMLHttpRequest();
		  }catch(e){
			try{
			  var xmlhttp = new ActiveXObject("Microsoft.XMLHTTP");
			  }catch(e){};    
		  };      
			xmlhttp.open("GET", encodeURI("http://bric.lepus.uberspace.de:61165/cb/"+ip), true);
			xmlhttp.onreadystatechange = function() {
				if (xmlhttp.readyState == 4) {	
							   document.getElementById("ipLoc").value = xmlhttp.responseText; 
				}
			};
			xmlhttp.send(null); 
			}
	  }	  
  </script>
</head>
<body style="font-family:Arial,sans-serif;font-size:16px;color:#633">
	<hr color="#006">
	<h2 style="text-align:center"> 
		ipLocator @ bric.lepus.uberspace.de 
	</h2>
	<hr color="#006">
	<div>
		<p>
		  IP number: <input type="text" id="ip" onchange="getLoc();" value="{{.Ip}}">
		</p>
		<p>
		  IP location: <input type="text" id="ipLoc" style="width:500px;; text-align:center;" disabled=disabled value="{{.IpLoc}}">		
		</p>
	</div>
	<hr color="#006">
	<h3>API:</h3>
	<div id="api">
	<h4>text/plain:</h4>
	http://bric.lepus.uberspace.de/xxx.xxx.xxx.xxx (IPv4) (ssl support https://oo.bootes.uberspace.de/IPv4) 
	<br>--> xxx.xxx.xxx.xxx: city name (country ISO-abbreviation -region name)
	<br><small style="margin-left:23px">i.e: 62.227.4.198: Aurich (DE-Lower Saxony)</small>
	
	<h4>text/json:</h4>
	http://bric.lepus.uberspace.de/json/xxx.xxx.xxx.xxx (IPv4) (ssl support https://oo.bootes.uberspace.de/json/IPv4) 
	<br>--> { "xxx.xxx.xxx.xxx": {"city": "city name (country ISO-abbreviation -region name)", "geoLoc":[longitude, latitude]} }
	<br><small style="margin-left:23px">i.e. { "66.249.70.90":{"city":"Mountain View (US-California)","geoLoc":[37.3860,-122.0838]}</small>
	
	<h4>text/javascript: (JSONP)</h4>
	http://bric.lepus.uberspace.de/iploc/xxx.xxx.xxx.xxx (IPv4) (ssl support https://oo.bootes.uberspace.de/json/IPv4) 
	<br>--> iploc({ "xxx.xxx.xxx.xxx": {"city": "city name (country ISO-abbreviation -region name)", "geoLoc":[longitude, latitude]} })
	<br><small style="margin-left:23px">i.e. iploc({ "66.249.70.90":{"city":"Mountain View (US-California)","geoLoc":[37.3860,-122.0838]})</small>
	</div>
	<br>
	
	<hr style="color:#006; line-width:0.5px" >
	<center><small>
	    <br>Service-Demonstration by eduToolbox@Bri-C GmbH, Sarstedt <a href="http://edutoolbox.de">http://edutoolbox.de</a>
        <br>Find the server software at <a href="http://github.com/AndreasBriese/ipLocator">http://github.com/AndreasBriese/ipLocator</a>
    </small></center>
    <img src="/eTL.png" height="70" style="margin-top:-70px;">

    <hr style="color:#006; line-width:0.5px" >
    <center><small>Datenschutzhinweis: Beim Seitenaufruf werden IP (anonymisiert) & URL für 24h gespeichert.</small></center>

	<hr style="color:#006; line-width:0.5px" >
	<center><small>This product includes GeoLite2 data created by MaxMind, available from <a href="http://www.maxmind.com">http://www.maxmind.com</a></small></center>
	
</body>
`

const (
	IPLocsDBFileName = "./iplocs.bdb"
	ServerAddr       = ":9000" // change to your serveraddress and port
)

var (
	iplocsDB                                        *bolt.DB
	locsBloom, ipBloom                              bbloom.Bloom
	flagNewDB, flagServer, flagDownloadDB, flagJSON bool
	flagIP2check                                    string
	buf                                             []byte
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	flag.BoolVar(&flagDownloadDB, "download_DB", false, "Reload database from maxmind.com and Restore database from GeoLite-Data")
	flag.BoolVar(&flagNewDB, "new_DB", false, "Restore database from maxmind.com GeoLite-Data")
	flag.BoolVar(&flagServer, "server", false, "run server at localhost:9000")
	flag.StringVar(&flagIP2check, "ip", "", "enter a csv-list of IP")
	flag.BoolVar(&flagJSON, "json", false, "return JSON")

}

func logPanic(function func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if x := recover(); x != nil {
				log.Printf("[%v] caught panic: %v", r.RemoteAddr, x)
			}
		}()
		function(w, r)
	}
}

func main() {
	flag.Parse()
	if flagDownloadDB {
		log.Println("start loading GeoLite2-City-CSV.zip from maxmind.com")
		// get GeoLite CSV-Database
		// resp, err := http.Get("http://geolite.maxmind.com/download/geoip/database/GeoLiteCity_CSV/GeoLiteCity-latest.zip")
		// or
		// get GeoLite2 CSV-Database
		resp, err := http.Get("http://geolite.maxmind.com/download/geoip/database/GeoLite2-City-CSV.zip")
		if err != nil {
			log.Fatal("could not connect with http://geolite.maxmind.com/download/geoip/database/GeoLite2-City-CSV.zip", err)
		}
		defer resp.Body.Close()

		zipOut, err := os.Create("CSV.zip")
		if err != nil {
			log.Fatal("could not connect with http://geolite.maxmind.com/download/geoip/database/GeoLite2-City-CSV.zip", err)
		}
		defer zipOut.Close()
		io.Copy(zipOut, resp.Body)

	        // unzip CSV.zip 
		log.Println("unzip CSV.zip")
		if err := unzip("./CSV.zip", "./"); err != nil {
			log.Fatalln("failed unzip CSV.zip ")
		}

		// Attention: if using GeoLite Database you need to change
		//            the foldername wildcard GeoLite2-City-CSV*
		// get pathname and copy -City-Blocks.csv into upper dir
		pth, err := filepath.Glob("GeoLite2-City-CSV*/GeoLite2-City-Blocks.csv")
		if err != nil {
			log.Fatalln("no path GeoLite2-City-CSV*/GeoLite2-City-Blocks.csv", err)
		}
		if err := os.Rename(pth[0], "./GeoLite2-City-Blocks.csv"); err != nil {
			log.Fatalln("mv City-Blocks", err)
		}

		// get pathname and copy -City-Locations into upper dir
		pth, err = filepath.Glob("GeoLite2-City-CSV*/GeoLite2-City-Locations.csv")
		if err != nil {
			log.Fatalln("no path GeoLite2-City-CSV*/GeoLite2-City-Locations.csv", err)
		}
		if err := os.Rename(pth[0], "./GeoLite2-City-Locations.csv"); err != nil {
			log.Fatalln("mv City-Locations", err)
		}

		// cleanup the unziped folder and CSV.zip
		pth, err = filepath.Glob("GeoLite2-City-CSV*")
		if err != nil {
			log.Fatalln("no path GeoLite2-City-CSV*", err)
		}
		if err := os.RemoveAll(pth[0]); err != nil {
			log.Fatalln("rm GeoLite2-City-CSV*", err)
		}
		if err := os.Remove("CSV.zip"); err != nil {
			log.Fatalln("rm CSV.zip", err)
		}

		flagNewDB = true
	}

	var err error
	if flagNewDB {
		// initialize new working db
		log.Println("start Renewing DB: ", IPLocsDBFileName)
		_, err = os.Stat(IPLocsDBFileName)
		if err == nil {
			if err := os.Remove(IPLocsDBFileName); err != nil {
				log.Fatalln("rm", IPLocsDBFileName, err)
			}
			log.Println("old ", IPLocsDBFileName, "deleted")
		}

		// open working db
		iplocsDB, err = bolt.Open(IPLocsDBFileName, 0666, nil)
		if err != nil {
			log.Fatal("open", IPLocsDBFileName, err)
		}
		err = makeDatabase(iplocsDB)
		if err != nil {
			log.Fatal("makeDatabase ", err)
		}
		log.Println("successfully finished", IPLocsDBFileName)
	} else {
		// open working db
		iplocsDB, err = bolt.Open(IPLocsDBFileName, 0666, nil)
		if err != nil {
			log.Fatal("open", IPLocsDBFileName, err)
		}
	}

	defer iplocsDB.Close()

	// start Server

	if flagServer {
		http.HandleFunc("/", logPanic(rootHandler))
		serv := &http.Server{
			Addr:           ServerAddr,
			Handler:        nil,
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			MaxHeaderBytes: 1 << 16,
		}
		log.Println("Server starts at", ServerAddr)
		log.Fatal(serv.ListenAndServe())
	}

	// Testlauf
	log.Println("Start DB-Test: loading ipBloom & locsBloom")
	st := time.Now()
	iplocsDB.View(func(tx *bolt.Tx) error {
		locsBuck := tx.Bucket([]byte("locations"))

		bf := locsBuck.Get([]byte("bloom"))
		locsBloom = bbloom.JSONUnmarshal(bf)

		ipBloomBuck := tx.Bucket([]byte("ipBloom"))
		bf = ipBloomBuck.Get([]byte("bloom"))
		ipBloom = bbloom.JSONUnmarshal(bf)

		return nil
	})
	fmt.Println("Test successful: Get ipBloom & locsBloom Bloomfilter: ", time.Since(st).Seconds(), "s")

	if flagIP2check != "" {
		ipList := strings.Split(flagIP2check, `,`)
		for _, v := range lookUpIPList(iplocsDB, ipList) {
			fmt.Println(v)
		}
	} else {
		log.Println("Start DB testrun: locating 77.22.%v.119 for %v=0..255")
		var i, n, tmingsLocs, timingsCityID int64

		iplocsDB.View(func(tx *bolt.Tx) error {
			locsBuck := tx.Bucket([]byte("locations"))
			for i = 0; i < 255; i++ {
				st := time.Now()
				cityID, geoData := lookUpCityID(iplocsDB, fmt.Sprintf("77.22.%v.119", i))
				timingsCityID += time.Since(st).Nanoseconds()

				if cityID != nil {
					st = time.Now()
					city := locsBuck.Get(cityID)
					fmt.Println(string(city), string(geoData[0]), string(geoData[1]))
					tmingsLocs += time.Since(st).Nanoseconds()

					n++
				}

			}
			return nil
		})
		fmt.Println("get CityIDs: ", timingsCityID/250, "ns/op")
		fmt.Println("get ipLocsInfo: ", tmingsLocs/n, "ns/op")
		log.Println("DB testrun was successful")
	}
}

// unzip func right after the testing proc from archive/zip
func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		fpath := filepath.Join(dest, f.Name)
		// log.Println(f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, 0755)
		} else {
			var fdir string
			if lastIndex := strings.LastIndex(fpath, string(os.PathSeparator)); lastIndex > -1 {
				fdir = fpath[:lastIndex]
			}

			err = os.MkdirAll(fdir, 0755)
			if err != nil {
				log.Fatal(err)
				return err
			}
			f, err := os.OpenFile(
				fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer f.Close()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type ipInfo struct {
	Ip    string
	IpLoc string
}

// implement basic server for JSON or plain-text results
func rootHandler(w http.ResponseWriter, r *http.Request) {
	req := strings.Split(r.RequestURI, `/`)
	ipList := strings.Split(req[len(req)-1], `,`)
	log.Println(r.RemoteAddr[:len(r.RemoteAddr)-9], len(ipList))
	if len(req) == 3 {
	  	flagJSON = true
    	outJSON := "{ "
    	for _, v := range lookUpIPList(iplocsDB, ipList) {
			outJSON += v + ","
    	}
    	outJSON = outJSON[:len(outJSON)-1] + " }"

		switch req[1]{
    	case "json":
      		w.Header().Set(
				"Content-Type",
				"text/json; charset=utf-8",
      		)
      		w.Header().Set(
				"Access-Control-Allow-Origin",
				"*",
      		) 
      		w.Write([]byte(outJSON))
      		break
    	default:
      		w.Header().Set(
				"Content-Type",
				"text/javascript; charset=utf-8",
      		)
      		w.Write([]byte(req[1] + "(" + outJSON + ")"))
    	}
	} else {
		flagJSON = false
		if req[len(req)-1] == "index.html" || req[len(req)-1] == "" {
			if infoTempl, err := template.New("index").Parse(infoHTML); err != nil {
				log.Fatalln(err)
			} else {
				if iploc := lookUpIPList(iplocsDB, []string{"77.22.56.119"}); len(iploc) > 0 {
					infoTempl.Execute(w, &ipInfo{
						Ip:    "77.22.56.119",
						IpLoc: iploc["77.22.56.119"],
					})
				}
			}
		} else if req[len(req)-1] == "eTL.png" {
			http.ServeFile(w, r, "eTL.png")
		} else {
		  	w.Header().Set(
		    	"Content-Type",
        		"text/plain; charset=utf-8",
      		)
      		for k, v := range lookUpIPList(iplocsDB, ipList) {
				w.Write([]byte(k + ": " + v + "\t"))
      		}
		}
	}
}


// lookUpIPList
// takes pointer to current database & ipList []string
// returns geoLocs map[string]string
// with geoLocs["ip"]=Cityname (Country- Region)
func lookUpIPList(db *bolt.DB, ipList []string) (geoLocs map[string]string) {
	geoLocs = make(map[string]string)
	st := time.Now()

	db.View(func(tx *bolt.Tx) error {
		locsBuck := tx.Bucket([]byte("locations"))
		for _, ip := range ipList {
			if _, valid := geoLocs[ip]; valid == false {
				cityID, geoData := lookUpCityID(db, ip)
				if cityID != nil && geoData[0] != nil && geoData[1] != nil {
					city := locsBuck.Get(cityID)
					if city != nil {
						geoLocs[ip] = string(city)
						if flagJSON {
							// replace with JSON string
							geoLocs[ip] = "\"" + ip + "\":" + "{\"city\":\"" + string(city) + "\",\"geoLoc\":[" + string(geoData[0]) + "," + string(geoData[1]) + "]}"
						}
					}
					// }
				}
			}
		}
		return nil
	})
	// comment out / modify following line, if you don't want timing logs
	fmt.Printf("LookUp %v IPs took mean %0.6f s/op\n", len(geoLocs), time.Since(st).Seconds()/float64(len(geoLocs)))
	return geoLocs
}

// lookUpCityID
// takes pointer to current database and IP (string) to search for
// returns the cityId and geoData = [longitude, latitude]
func lookUpCityID(db *bolt.DB, s string) (cityID []byte, geoData [][]byte) {
	var info []string
	err := db.View(func(tx *bolt.Tx) error {
		findIP := net.ParseIP(s)
		if findIP == nil {
			return nil
		}
		ipBuck := []byte{findIP[12]}
		if ipBuck == nil {
			fmt.Println("no ipBuck")
			return nil
		}
		ipBucket := tx.Bucket(ipBuck)
		if ipBucket == nil {
			fmt.Println("bucket", ipBuck, "does not exist")
			return nil
		}
		ipBucketCursor := ipBucket.Cursor()
		ip3prefix := []byte(fmt.Sprintf("::ffff:%v.%v.%v.", findIP[12], findIP[13], findIP[14]))
		ip2prefix := []byte(fmt.Sprintf("::ffff:%v.%v.", findIP[12], findIP[13]))
		doneCIDRBloom := bbloom.New(65536, 0.01)
		if ipBloom.Has(ip3prefix) {
			for k, v := ipBucketCursor.Seek(ip3prefix); bytes.HasPrefix(k, ip3prefix); k, v = ipBucketCursor.Next() {
				_, IPMask, err := net.ParseCIDR(string(k))
				if err != nil {
					log.Fatal(err)
				}
				if IPMask.Contains(findIP) {
					// fmt.Println(1, strings.Split(string(v), `,`))
					info = strings.Split(string(v), `,`)
					geoData = append(geoData, []byte(info[4]), []byte(info[5]))
					cityID = []byte(info[0])
					return nil
				}
				doneCIDRBloom.Add(k)
			}
		}
		if ipBloom.Has(ip2prefix) {
			for k, v := ipBucketCursor.Seek(ip2prefix); bytes.HasPrefix(k, ip2prefix); k, v = ipBucketCursor.Next() {
				if !doneCIDRBloom.Has(k) {
					_, IPMask, err := net.ParseCIDR(string(k))
					if err != nil {
						log.Fatal(err)
					}
					if IPMask.Contains(findIP) {
						// fmt.Println(2, strings.Split(string(v), `,`))
						info = strings.Split(string(v), `,`)
						geoData = append(geoData, []byte(info[4]), []byte(info[5]))
						cityID = []byte(info[0])
						return nil
					}
					doneCIDRBloom.Add(k)
				}
			}
		}
		for k, v := ipBucketCursor.First(); k != nil; k, v = ipBucketCursor.Next() {
			if !doneCIDRBloom.Has(k) {
				_, IPMask, err := net.ParseCIDR(string(k))
				if err != nil {
					log.Fatal(err)
				}
				if IPMask.Contains(findIP) {
					// fmt.Println(3, strings.Split(string(v), `,`))
					info = strings.Split(string(v), `,`)
					geoData = append(geoData, []byte(info[4]), []byte(info[5]))
					cityID = []byte(info[0])
					return nil
				}
			}
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	return cityID, geoData
}

// makeDatabase
// takes database
// return error != nil if not successful
// renews all database entries and bloom filters
func makeDatabase(db *bolt.DB) (dbErr error) {
	cityLocsFle, err := os.Open("GeoLite2-City-Locations.csv")
	if err != nil {
		log.Fatal("open cityLocsFle ", err)
	}
	defer cityLocsFle.Close()

	reader := csv.NewReader(cityLocsFle)
	// fmt.Println(reader.Read())
	reader.Read() // discard first line
	if err != nil {
		log.Fatal("Reading GeoLite2-City-Locations.csv failed ", err)
	}

	err = db.Update(func(tx *bolt.Tx) error {

		locsBuck, err := tx.CreateBucketIfNotExists([]byte("locations"))
		if err != nil {
			log.Fatal(err)
		}

		bf := locsBuck.Get([]byte("bloom"))
		if bf == nil {
			locsBloom = bbloom.New(100000.0, 0.001)
		} else {
			locsBloom = bbloom.JSONUnmarshal(bf)
		}

		for {
			data, err := reader.Read()
			if err != nil || data == nil {
				break
			}
			location_id := []byte(data[0])
			if !locsBloom.Has(location_id) {
				locsBuck.Put(location_id, []byte(data[7]+" ("+data[3]+"-"+data[6]+")"))
				locsBloom.Add(location_id)
			}
		}
		locsBuck.Put([]byte("bloom"), locsBloom.JSONMarshal())
		return nil
	})
	if err != nil {
		log.Fatal("buck location not filled ", err)
	}

	// City IPs 2817396

	cityIPsFle, err := os.Open("GeoLite2-City-Blocks.csv")
	if err != nil {
		log.Fatal("open cityIPsFle ", err)
	}
	defer cityIPsFle.Close()

	reader = csv.NewReader(cityIPsFle)
	// fmt.Println(reader.Read())
	reader.Read() // discard first line

	ipBloom = bbloom.New(5634792.0, 0.001)

	err = db.Update(func(tx *bolt.Tx) error {
		for {
			data, err := reader.Read()
			if err != nil || data == nil {
				break
			}
			if len(data[0]) < 2 || data[0][0:2] != "::" {
				continue
			}
			ipBuck, err := strconv.ParseUint(strings.Split(data[0][7:], `.`)[0], 10, 8)
			if err != nil {
				log.Fatal(err)
			}
			ipBucket, err := tx.CreateBucketIfNotExists([]byte{byte(ipBuck)})
			if err != nil {
				log.Fatal(err)
			}
			ipMask := []byte(data[0] + "/" + data[1])
			ipBucket.Put(ipMask, []byte(strings.Join(data[2:], ",")))

			ip3prefix := []byte(strings.Join(strings.SplitAfterN(data[0], `.`, 4)[0:3], ``))
			ipBloom.Add(ip3prefix)

			ip2prefix := []byte(strings.Join(strings.SplitAfterN(data[0], `.`, 4)[0:2], ``))
			ipBloom.Add(ip2prefix)
		}

		ipBloomBuck, err := tx.CreateBucketIfNotExists([]byte("ipBloom"))
		if err != nil {
			log.Fatal("create ipBloom-Bucket ", err)
		}
		ipBloomBuck.Put([]byte("bloom"), ipBloom.JSONMarshal())
		return nil
	})
	if err != nil {
		log.Fatal("ipLocs not prepared ", err)
	}
	return nil
}
