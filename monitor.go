package main

import (
	"context"
	"fmt"
	"time"

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
	sh := shell.NewShell("localhost:5001")

	ticker := time.NewTicker(2 * time.Second) //ticker will have a channel C inside it
	defer ticker.Stop()                       //ticker will stop before the main func exits
	done := make(chan bool)                   //creates the "done" channel
	go func() {                               //writes true in the channel after x time to stop the execution of the ticker
		time.Sleep(15 * time.Minute)
		done <- true
	}()

	var collectedData plotter.XYs

	for { //while true
		select { //select lets a routine wait on multiple communication operations
		case <-done:
			fmt.Println("Done!")
			fmt.Println(collectedData)
			plotData(collectedData)
			return // causes the main to exit
		case _ = <-ticker.C: //if something is written on the ticker channel... (this happens every two seconds)
			{
				cid, err := sh.SwarmPeers(context.Background())
				if err != nil {
					panic(err)
				}
				fmt.Println(len(cid.Peers)) // [ERROR]cid.Peers is always the same value!!
				collectedData = append(collectedData, plotter.XY{float64(time.Now().Unix()), float64(len(cid.Peers))})
				//{strconv.ParseFloat(t.String(), 64), strconv.ParseFloat(len(cid.Peers), 64)}
			}
		}
	}
}
