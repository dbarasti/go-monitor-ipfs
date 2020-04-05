package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"

	"gonum.org/v1/plot/plotutil"

	shell "github.com/ipfs/go-ipfs-api"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

type data struct {
	Timestamp   time.Time
	PeersNumber int
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

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file")
	}
	//extract in function
	sampleFrequency, err := strconv.ParseInt(os.Getenv("SAMPLE_FREQUENCY_SEC"), 10, 64)
	if err != nil {
		panic(err)
	}
	sampleTime, err := strconv.ParseInt(os.Getenv("SAMPLE_TIME_MIN"), 10, 64)
	if err != nil {
		panic(err)
	}
	//extract in function

	sh := shell.NewShell(os.Getenv("IPFS_SERVER_PORT"))

	ticker := time.NewTicker(time.Duration(sampleFrequency) * time.Second) //ticker will have a channel C inside it
	defer ticker.Stop()                                                    //ticker will stop before the main func exits
	done := make(chan bool)                                                //creates the "done" channel
	go func() {                                                            //writes true in the channel after x time to stop the execution of the ticker
		time.Sleep(time.Duration(sampleTime) * time.Minute)
		done <- true
	}()

	var collectedData plotter.XYs

	for { //while true
		select { //select lets a routine wait on multiple communication operations
		case <-done:
			fmt.Println("Done!")
			plotData(collectedData)
			return // causes the main to exit
		case _ = <-ticker.C: //if something is written on the ticker channel... (this happens every two seconds)
			{
				sh.
				cid, err := sh.SwarmPeers(context.Background())
				if err != nil {
					panic(err)
				}
				fmt.Println(len(cid.Peers))
				collectedData = append(collectedData, plotter.XY{float64(time.Now().Unix()), float64(len(cid.Peers))})
				//{strconv.ParseFloat(t.String(), 64), strconv.ParseFloat(len(cid.Peers), 64)}
			}
		}
	}
}
