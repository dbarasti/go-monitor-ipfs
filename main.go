package main

import (
	"ipfs_monitoring/bwmonitor"
	"sync"
)

func main() {
	var wg sync.WaitGroup
	wg.Add(1)
	go bwmonitor.RunMonitor(&wg)
	wg.Wait()
	/*
		if err := ipstack.Init(os.Getenv("IPSTACK_KEY")); err != nil {
			panic(err)
		}
		if res, err := ipstack.IP("89.166.211.133"); err == nil {
			fmt.Printf("IP: %s, city: %s, country: %s (%s)\n", res.IP, res.City, res.CountryName, res.CountryCode)
		}
	*/
}
