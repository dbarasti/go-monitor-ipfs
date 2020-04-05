package main

import (
	"ipfs_monitoring/bwmonitor"
	"log"
	"os"
	"sync"
)

func main() {
	file := setupLog()
	defer file.Close()
	log.Print("[MAIN] Starting execution")

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

func setupLog() *os.File {
	file, err := os.OpenFile("info.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(file)
	return file
}
