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
	TotalIn  int64   `json:"TotalIn"`
	TotalOut int64   `json:"TotalOut"`
	RateIn   float64 `json:"RateIn"`
	RateOut  float64 `json:"RateOut"`
}

//ToArray turns each field into a string and returns a slice of these strings that a csv.Writer will accept.
func (i *bwInfo) ToArray() []string {
	var arrayData []string
	arrayData = append(arrayData, fmt.Sprintf("%d", i.TotalOut))
	arrayData = append(arrayData, fmt.Sprintf("%d", i.TotalIn))
	arrayData = append(arrayData, fmt.Sprintf("%f", i.RateIn))
	arrayData = append(arrayData, fmt.Sprintf("%f", i.RateOut))
	return arrayData
}

func cleanBwRequest(r *http.Response) bwInfo {
	b, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		panic(err)
	}

	// Unmarshal
	var msg bwInfo
	err = json.Unmarshal(b, &msg)
	if err != nil {
		panic(err)
	}
	return msg
}

//SampleBandwidth uses ipfs http API to retrieve current information about bandwidth usage
func sampleBandwidth() bwInfo {
	ipfsServer := os.Getenv("IPFS_SERVER_PORT")
	if ipfsServer == "" {
		fmt.Fprintf(os.Stderr, "error: undefined variable IPFS_SERVER_PORT in environment")
		os.Exit(1)
	}
	r, err := http.Get("http://" + ipfsServer + "/api/v0/stats/bw")
	if err != nil {
		panic(err)
	}
	msg := cleanBwRequest(r)
	fmt.Println(msg)
	return msg
}

func writeToCsv(data *[]bwInfo) error {
	file, err := os.Create("result.csv")
	checkError("Cannot create file", err)
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()
	for _, value := range *data {
		err := writer.Write(value.ToArray())
		checkError("Cannot write to file", err)
	}
	return nil
}

func checkError(message string, err error) {
	if err != nil {
		log.Fatal(message, err)
	}
}

//RunMonitor starts monitoring the bandwidth at a specific rate then writes data in a csv file
func RunMonitor(wg *sync.WaitGroup) {
	defer wg.Done()
	err := godotenv.Load(".env")
	checkError("Error loading .env file", err)
	sampleFrequence, err := strconv.ParseInt(os.Getenv("SAMPLE_FREQUENCE_SEC"), 10, 64)
	checkError("SAMPLE_FREQUENCE_SEC not found in .env file", err)
	sampleTime, err := strconv.ParseInt(os.Getenv("SAMPLE_TIME_MIN"), 10, 64)
	checkError("SAMPLE_TIME_MIN not found in .env file", err)

	ticker := time.NewTicker(time.Duration(sampleFrequence) * time.Second)
	done := make(chan bool)
	var data []bwInfo
	go func() {
		for {
			select {
			case <-done:
				return
			case t := <-ticker.C:
				fmt.Println("Sample at", t.Format("03:04:05 PM"))
				data = append(data, sampleBandwidth())
			}
		}
	}()

	time.Sleep(time.Duration(sampleTime) * time.Second)
	ticker.Stop()
	done <- true
	writeToCsv(&data)
	fmt.Println("bw monitoring stopped")
}
