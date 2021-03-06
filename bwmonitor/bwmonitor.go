package bwmonitor

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

type bwInfo struct {
	Time     time.Time
	TotalIn  int64   `json:"TotalIn"`
	TotalOut int64   `json:"TotalOut"`
	RateIn   float64 `json:"RateIn"`
	RateOut  float64 `json:"RateOut"`
}

//ToArray turns each field into a string and returns a slice of these strings that a csv.Writer will accept.
func (i *bwInfo) ToArray() []string {
	var arrayData []string
	arrayData = append(arrayData, fmt.Sprintf("%s", i.Time.Format("2 Jan 2006 15:04:05")))
	arrayData = append(arrayData, fmt.Sprintf("%d", i.TotalOut))
	arrayData = append(arrayData, fmt.Sprintf("%d", i.TotalIn))
	arrayData = append(arrayData, fmt.Sprintf("%f", i.RateIn))
	arrayData = append(arrayData, fmt.Sprintf("%f", i.RateOut))
	return arrayData
}

func cleanBwRequest(r *http.Response) (bwInfo, error) {
	b, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		log.Print("[BWMONITOR] Cannot read request body", err)
		return bwInfo{}, err
	}
	// Unmarshal
	var msg bwInfo
	err = json.Unmarshal(b, &msg)
	if err != nil {
		panic(err)
	}
	return msg, nil
}

//sampleBandwidth uses ipfs http API to retrieve information about current bandwidth usage
func sampleBandwidth(t time.Time) (bwInfo, error) {
	ipfsServer := os.Getenv("IPFS_SERVER_PORT")
	if ipfsServer == "" {
		log.Fatal("[BWMONITOR] error: undefined variable IPFS_SERVER_PORT in environment")
	}
	r, err := http.Get("http://" + ipfsServer + "/api/v0/stats/bw")
	if err != nil {
		return bwInfo{}, err
	}
	bwData, err := cleanBwRequest(r)
	bwData.Time = t.Round(1 * time.Second)
	return bwData, err
}

func writeToCsv(data *[]bwInfo) error {
	file, err := os.Create("bw-info.csv")
	if err != nil {
		return err
	}

	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()
	for _, value := range *data {
		err := writer.Write(value.ToArray())
		if err != nil {
			log.Print("[BWMONITOR] warning: cannot write value to file:", err)
		}
	}
	return nil
}

func checkFatalError(message string, err error) {
	if err != nil {
		log.Fatal(message, err)
	}
}

//RunMonitor starts monitoring the bandwidth at a specific rate. Then writes data in a csv file
func RunMonitor(wg *sync.WaitGroup) {
	log.Print("[BWMONITOR] Starting bwmonitor")
	defer log.Print("[BWMONITOR] End of monitoring")
	defer wg.Done()
	sampleFrequency, measurementTime := getSamplingVariables()
	log.Print("[BWMONITOR] End scheduled for ", time.Now().Add(time.Minute*time.Duration(measurementTime)).Format("2 Jan 2006 15:04:05"))

	ticker := time.NewTicker(time.Duration(sampleFrequency) * time.Second)
	defer ticker.Stop()
	done := make(chan bool)
	var data []bwInfo
	go func() {
		for {
			select {
			case <-done:
				return
			case t := <-ticker.C:
				if bwData, err := sampleBandwidth(t); err == nil {
					data = append(data, bwData)
				} else {
					log.Print("[BWMONITOR] Error during request for bw stats:", err)
				}
			}
		}
	}()

	time.Sleep(time.Duration(measurementTime) * time.Minute)

	done <- true
	err := writeToCsv(&data)
	if err != nil {
		log.Print("[BWMONITOR] error while writing data to file")
	}
}

func getSamplingVariables() (int64, int64) {
	err := godotenv.Load(".env")
	checkFatalError("[SWARM_MONITOR] Error loading .env file", err)
	sampleFrequency, err := strconv.ParseInt(os.Getenv("SAMPLE_FREQUENCY_SEC"), 10, 64)
	checkFatalError("[SWARM_MONITOR] SAMPLE_FREQUENCY_SEC not found in .env file:", err)
	measurementTime, err := strconv.ParseInt(os.Getenv("SAMPLE_TIME_MIN"), 10, 64)
	checkFatalError("[SWARM_MONITOR] SAMPLE_TIME_MIN not found in .env file:", err)
	return sampleFrequency, measurementTime
}
