package swarmmonitor

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ipinfo/go-ipinfo/ipinfo"

	"github.com/joho/godotenv"

	shell "github.com/ipfs/go-ipfs-api"
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

func ipInfoToArray(ipNfo *ipinfo.Info) []string {
	arr := make([]string, 0)
	arr = append(arr, fmt.Sprintf("%s", ipNfo.IP))
	arr = append(arr, fmt.Sprintf("%s", ipNfo.Country))
	arr = append(arr, fmt.Sprintf("%s", ipNfo.City))
	return arr
}

var lock = sync.RWMutex{}
var ipDatabase = make(map[string]*ipinfo.Info)

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

func writeIpInfoToCsv(database map[string]*ipinfo.Info) error {
	file, err := os.Create("ip-info.csv")
	if err != nil {
		return err
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()
	for _, value := range database {
		err := writer.Write(ipInfoToArray(value))
		if err != nil {
			log.Print("[SWARM_MONITOR] warning: cannot write value to file:", err)
		}
	}
	return nil
}

//RunMonitor starts monitoring the swarm at a specific rate. Then writes data in a csv file
func RunMonitor(wg *sync.WaitGroup) {
	log.Print("[SWARM_MONITOR] Starting bwmonitor")
	defer log.Print("[SWARM_MONITOR] End of monitoring")
	defer wg.Done()
	sampleFrequency, measurementTime := getSamplingVariables()
	log.Print("[SWARM_MONITOR] End scheduled for ", time.Now().Add(time.Minute*time.Duration(measurementTime)).Format("2 Jan 2006 15:04:05"))

	sh := shell.NewShell(os.Getenv("IPFS_SERVER_PORT"))
	ticker := time.NewTicker(time.Duration(sampleFrequency) * time.Second)
	defer ticker.Stop()
	done := make(chan bool)

	activePeers := make(map[string]*PeerInfo)
	pastConnections := make(map[string][]*PeerInfo)
	samplesInfo := make([]SampleInfo, 0)
	ipLookupJobs := make(chan *PeerInfo)

	go func() {
		for {
			select {
			case <-done:
				return
			case t := <-ticker.C:
				swarmInfo, err := sh.SwarmPeers(context.Background())
				if err != nil {
					log.Print("[SWARM_MONITOR] Error while getting swarm peers: ", err)
				}
				//create multiple workers
				for i := 0; i < 10; i++ {
					go findIPLocation(ipLookupJobs, ipDatabase)
				}

				for _, peerInfo := range swarmInfo.Peers {
					handleSample(t.Round(1*time.Second), peerInfo, activePeers)
				}
				cleanActiveSwarms(activePeers, pastConnections, ipLookupJobs)
				samplesInfo = append(samplesInfo, SampleInfo{
					Timestamp:   t.Round(1 * time.Second),
					PeersNumber: len(swarmInfo.Peers),
				})
			}
		}
	}()

	time.Sleep(time.Duration(measurementTime) * time.Minute)
	done <- true
	closeActiveConnections(activePeers, pastConnections, ipLookupJobs)

	time.Sleep(15 * time.Second)

	if err := writeConnectionsToCsv(pastConnections); err != nil {
		log.Print("[SWARM_MONITOR] error while writing data to file")
	}
	if err := writeSamplesInfoToCsv(samplesInfo); err != nil {
		log.Print("[SWARM_MONITOR] error while writing data to file")
	}

	if err := writeIpInfoToCsv(ipDatabase); err != nil {
		log.Print("[SWARM_MONITOR] error while writing data to file")
	}
}

func closeActiveConnections(peers map[string]*PeerInfo, connections map[string][]*PeerInfo, jobs chan *PeerInfo) {
	for _, peerInfo := range peers {
		connections[peerInfo.Id] = append(connections[peerInfo.Id], peerInfo)
		jobs <- peerInfo
		delete(peers, peerInfo.Id)
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

func cleanActiveSwarms(activePeers map[string]*PeerInfo, pastConnections map[string][]*PeerInfo, jobs chan *PeerInfo) {
	for id, peer := range activePeers {
		if peer.Updated == false {
			(pastConnections)[id] = append((pastConnections)[id], peer)
			jobs <- peer
			delete(activePeers, id)
		}
		peer.Updated = false
	}
}

func handleSample(t time.Time, peerInfo shell.SwarmConnInfo, activeSwarm map[string]*PeerInfo) {
	peer, ok := activeSwarm[peerInfo.Peer]
	if ok == true {
		peer.LastSeen = t
		peer.Updated = true
	} else {
		ip := strings.Split(peerInfo.Addr, "/")[2]
		activeSwarm[peerInfo.Peer] = &PeerInfo{peerInfo.Peer, ip, t, t, true, ""}
	}
}

func findIPLocation(jobs chan *PeerInfo, ipDatabase map[string]*ipinfo.Info) {
	authTransport := ipinfo.AuthTransport{Token: os.Getenv("IPSTACK_KEY")}
	httpClient := authTransport.Client()
	client := ipinfo.NewClient(httpClient)

	for {
		select {
		case job := <-jobs:
			lock.RLock()
			ip, ok := ipDatabase[job.Ip]
			lock.RUnlock()
			if ok == false {
				//log.Print("[SWARM_MONITOR] finding location of ", job.Ip)
				if res, err := client.GetInfo(net.ParseIP(job.Ip)); err == nil { //todo measure time for request and sleep if too short
					//log.Print("[SWARM_MONITOR] location of ", job.Ip, ":", res.Country)
					job.Location = res.Country
					lock.RLock()
					ipDatabase[job.Ip] = res
					lock.RUnlock()
				} else {
					log.Print("[SWARM_MONITOR] Error finding location for ", job.Ip, ":", err)
				}
			} else {
				job.Location = ip.Country
				//log.Print("Information for ", job.Ip, " already present in DB")
			}
		}
	}
}

func checkFatalError(message string, err error) {
	if err != nil {
		log.Fatal(message, err)
	}
}
