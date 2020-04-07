package swarmmonitor

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/qioalice/ipstack"

	"github.com/joho/godotenv"

	"gonum.org/v1/plot/plotutil"

	shell "github.com/ipfs/go-ipfs-api"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

type SampleInfo struct {
	Timestamp   time.Time
	PeersNumber int
}

func (i *SampleInfo) ToArray() []string {
	var arrayData []string
	arrayData = append(arrayData, fmt.Sprintf("%s", i.Timestamp.Format("2 Jan 2006 15:04:05")))
	arrayData = append(arrayData, fmt.Sprintf("%d", i.PeersNumber))
	return arrayData
}

type PeerInfo struct {
	Id        string
	Ip        string
	FirstSeen time.Time
	LastSeen  time.Time
	Updated   bool
	Location  string //Country Name
}

func (i *PeerInfo) ToArray() []string {
	var arrayData []string
	arrayData = append(arrayData, fmt.Sprintf("%s", i.Id))
	arrayData = append(arrayData, fmt.Sprintf("%s", i.Ip))
	arrayData = append(arrayData, fmt.Sprintf("%s", i.FirstSeen.Format("2 Jan 2006 15:04:05")))
	arrayData = append(arrayData, fmt.Sprintf("%s", i.LastSeen.Format("2 Jan 2006 15:04:05")))
	arrayData = append(arrayData, fmt.Sprintf("%v", i.Updated))
	arrayData = append(arrayData, fmt.Sprintf("%s", i.Location))
	return arrayData
}

func writeConnectionsToCsv(pastConnections map[string][]*PeerInfo) error {
	file, err := os.Create("connections-info.csv")
	if err != nil {
		return err
	}

	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()
	for _, peerConnections := range pastConnections {
		for _, peerConnection := range peerConnections {
			err := writer.Write(append(peerConnection.ToArray(), fmt.Sprintf("%s", peerConnection.LastSeen.Sub(peerConnection.FirstSeen))))
			if err != nil {
				log.Print("[SWARM_MONITOR] warning: cannot write value to file:", err)
			}
		}
		//log.Printf("[SWARM_MONITOR] Connections of %s: %v", id, peerConnections)
	}
	return nil
}

func writeSamplesInfoToCsv(samplesInfo []SampleInfo) error {
	file, err := os.Create("samples-info.csv")
	if err != nil {
		return err
	}

	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()
	for _, value := range samplesInfo {
		err := writer.Write(value.ToArray())
		if err != nil {
			log.Print("[SWARM_MONITOR] warning: cannot write value to file:", err)
		}
	}
	return nil
}

func plotData(collectedData plotter.XYs) {
	fmt.Println("creating plot...")
	p, err := plot.New()
	if err != nil {
		panic(err)
	}
	p.Title.Text = "swarm peers over time"
	p.X.Label.Text = "time"
	p.Y.Label.Text = "#peers"

	err = plotutil.AddLinePoints(p, "numbers", collectedData)

	if err != nil {
		panic(err)
	}

	if err := p.Save(4*vg.Inch, 4*vg.Inch, "peers_analysis.png"); err != nil {
		panic(err)
	}
}

//RunMonitor starts monitoring the swarm at a specific rate. Then writes data in a csv file
func RunMonitor(wg *sync.WaitGroup) {
	log.Print("[SWARM_MONITOR] Starting bwmonitor")
	defer log.Print("[SWARM_MONITOR] End of monitoring")
	defer wg.Done()
	sampleFrequency, measurementTime := getSamplingVariables()
	log.Print("[SWARM_MONITOR] End scheduled for ", time.Now().Add(time.Minute*time.Duration(measurementTime)).Format("2 Jan 2006 15:04:05"))

	sh := shell.NewShell(os.Getenv("IPFS_SERVER_PORT"))
	ticker := time.NewTicker(time.Duration(sampleFrequency) * time.Second) //ticker will have a channel C inside it
	defer ticker.Stop()                                                    //ticker will stop before the main func exits
	done := make(chan bool)                                                //creates the "done" channel

	activePeers := make(map[string]*PeerInfo)
	pastConnections := make(map[string][]*PeerInfo)
	samplesInfo := make([]SampleInfo, 0)
	go func() { //writes true in the channel after x time to stop the execution of the ticker
		for {
			select { //select lets a routine wait on multiple communication operations
			case <-done:
				return
			case t := <-ticker.C: //if something is written on the ticker channel... (this happens every two seconds)
				swarmInfo, err := sh.SwarmPeers(context.Background())
				if err != nil {
					log.Print("[SWARM_MONITOR] Error while getting swarm peers: ", err)
				}
				ipLookupJobs := make(chan *PeerInfo)
				//create multiple workers
				for i := 0; i < 10; i++ {
					go findIPLocation(ipLookupJobs, done)
				}

				for _, peerInfo := range swarmInfo.Peers {
					handleSample(t, peerInfo, activePeers, ipLookupJobs)
				}
				cleanActiveSwarms(activePeers, &pastConnections)
				samplesInfo = append(samplesInfo, SampleInfo{
					Timestamp:   t,
					PeersNumber: len(swarmInfo.Peers),
				})
			}
		}
	}()

	time.Sleep(time.Duration(measurementTime) * time.Minute)
	done <- true

	if err := writeConnectionsToCsv(pastConnections); err != nil {
		log.Print("[SWARM_MONITOR] error while writing data to file")
	}
	if err := writeSamplesInfoToCsv(samplesInfo); err != nil {
		log.Print("[SWARM_MONITOR] error while writing data to file")
	}
	//todo move all data in activePeers into pastConnections
	//plotData(collectedData)

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

func cleanActiveSwarms(activePeers map[string]*PeerInfo, pastConnections *map[string][]*PeerInfo) {
	for id, peer := range activePeers {
		if peer.Updated == false {
			(*pastConnections)[id] = append((*pastConnections)[id], peer)
			delete(activePeers, id)
		}
		peer.Updated = false
	}
}

func handleSample(t time.Time, peerInfo shell.SwarmConnInfo, activeSwarm map[string]*PeerInfo, jobs chan *PeerInfo) {
	peer, ok := activeSwarm[peerInfo.Peer]
	if ok == true {
		peer.LastSeen = t
		peer.Updated = true
	} else {
		ip := strings.Split(peerInfo.Addr, "/")[2]
		activeSwarm[peerInfo.Peer] = &PeerInfo{peerInfo.Peer, ip, t, t, true, ""}
		// todo temporary off: jobs <- activeSwarm[peerInfo.Peer]
	}
}

func findIPLocation(jobs chan *PeerInfo, done chan bool) {
	for {
		select {
		case job := <-jobs: //throws seg error at the end
			fmt.Println("[SWARM_MONITOR] finding location of", job.Ip)
			if err := ipstack.Init(os.Getenv("IPSTACK_KEY")); err != nil {
				log.Print("[SWARM_MONITOR] IPSTACK_KEY not found in .env file:", err)
			}
			if res, err := ipstack.IP(job.Ip); err == nil {
				fmt.Println("[SWARM_MONITOR] location of", job.Ip, ":", res.CountryCode)
				job.Location = res.CountryCode
			} else {
				log.Print("[SWARM_MONITOR] Error finding location for ", job.Ip, ":", err)
			}
		}
	}
}

func checkFatalError(message string, err error) {
	if err != nil {
		log.Fatal(message, err)
	}
}
