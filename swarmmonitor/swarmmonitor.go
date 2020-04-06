package swarmmonitor

import (
	"context"
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

type PeerInfo struct {
	Id        string
	Ip        string
	FirstSeen time.Time
	LastSeen  time.Time
	Updated   bool
	Location  string //Country Name
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
	defer log.Print("[SWARM_MONITOR] end of monitoring")
	defer wg.Done()
	err := godotenv.Load(".env")
	checkFatalError("[SWARM_MONITOR] Error loading .env file", err)
	sampleFrequency, err := strconv.ParseInt(os.Getenv("SAMPLE_FREQUENCY_SEC"), 10, 64)
	checkFatalError("[SWARM_MONITOR] SAMPLE_FREQUENCY_SEC not found in .env file:", err)
	sampleTime, err := strconv.ParseInt(os.Getenv("SAMPLE_TIME_MIN"), 10, 64)
	checkFatalError("[SWARM_MONITOR] SAMPLE_TIME_MIN not found in .env file:", err)
	log.Print("[SWARM_MONITOR] End scheduled for ", time.Now().Add(time.Minute*time.Duration(sampleTime)).Format("2 Jan 2006 15:04"))

	sh := shell.NewShell(os.Getenv("IPFS_SERVER_PORT"))
	ticker := time.NewTicker(time.Duration(sampleFrequency) * time.Second) //ticker will have a channel C inside it
	defer ticker.Stop()                                                    //ticker will stop before the main func exits
	done := make(chan bool)                                                //creates the "done" channel

	activePeers := make(map[string]*PeerInfo)
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
				for _, peerInfo := range swarmInfo.Peers {
					handleSample(t, peerInfo, activePeers)
				}
				//fmt.Println("[SWARM_MONITOR]", len(swarmInfo.Peers), "peers")

				//collectedData = append(collectedData, plotter.XY{float64(time.Now().Unix()), float64(len(swarmInfo.Peers))})
				//{strconv.ParseFloat(t.String(), 64), strconv.ParseFloat(len(swarmInfo.Peers), 64)}
			}
		}
	}()

	time.Sleep(time.Duration(sampleTime) * time.Minute)
	done <- true
	//plotData(collectedData)

}

func handleSample(t time.Time, peerInfo shell.SwarmConnInfo, active map[string]*PeerInfo) {
	peer, ok := active[peerInfo.Peer]
	if ok == true {
		peer.LastSeen = t
		peer.Updated = true
	} else {
		ip := strings.Split(peerInfo.Addr, "/")[2]
		active[peerInfo.Peer] = &PeerInfo{peerInfo.Peer, ip, t, t, true, ""}
		ipLocation := findIpLocation(ip)
		active[peerInfo.Peer].Location = ipLocation
	}
}

//warning!! serve rendere asincrona la ricerca dell'ip altrimenti l'app si blocca qui per molto tempo durante il primo sampling
func findIpLocation(ip string) string {
	fmt.Println("[SWARM_MONITOR] finding location of", ip)
	if err := ipstack.Init(os.Getenv("IPSTACK_KEY")); err != nil {
		log.Print("[SWARM_MONITOR] IPSTACK_KEY not found in .env file:", err)
		return ""
	}
	if res, err := ipstack.IP(ip); err == nil {
		fmt.Println("[SWARM_MONITOR] location of", ip, ":", res.CountryCode)
		return res.CountryCode
	} else {
		log.Print("[SWARM_MONITOR] Error finding location for ", ip, ":", err)
		return ""
	}
}

func checkFatalError(message string, err error) {
	if err != nil {
		log.Fatal(message, err)
	}
}
