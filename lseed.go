package main

import (
	"flag"
	"os/user"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/cdecker/kugelblitz/lightningrpc"
	"github.com/cdecker/lightning-seed/seed"
)

var (
	lightningRpc *lightningrpc.LightningRpc

	listenPort    = flag.Int("port", 53, "Port to listen for incoming requests.")
	pollInterval  = flag.Int("poll-interval", 10, "Time between polls to lightningd for updates")
	lightningSock = flag.String("lightning-sock", "$HOME/.lightning/lightning-rpc", "Location of the lightning socket")
	debug         = flag.Bool("debug", false, "Be very verbose")
	numResults    = flag.Int("results", 25, "How many results shall we return to a query?")
)

// Expand variables in paths such as $HOME
func expandVariables() error {
	user, err := user.Current()
	if err != nil {
		return err
	}
	*lightningSock = strings.Replace(*lightningSock, "$HOME", user.HomeDir, -1)
	return nil
}

// Regularly polls the lightningd node and updates the local NetworkView.
func poller(lrpc *lightningrpc.LightningRpc, nview *seed.NetworkView, wg *sync.WaitGroup) {
	defer func() { wg.Done() }()
	for {
		r, err := lrpc.GetNodes()
		if err != nil {
			log.Errorf("Error trying to get update from lightningd: %v", err)
		} else {
			log.Debugf("Got %d nodes from lightningd", len(r.Nodes))
			for _, n := range r.Nodes {
				if n.Ip == "" || n.Port <= 1024 {
					continue
				}
				nview.AddNode(n)
			}
		}
		time.Sleep(time.Second * time.Duration(*pollInterval))
	}
}

// Parse flags and configure subsystems according to flags
func configure() {
	flag.Parse()
	expandVariables()
	if *debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
}

// Main entry point for the lightning-seed
func main() {
	var wg sync.WaitGroup
	configure()
	lightningRpc = lightningrpc.NewLightningRpc(*lightningSock)

	nview := seed.NewNetworkView()
	wg.Add(3)
	dnsServer := seed.NewDnsServer(nview)

	go poller(lightningRpc, nview, &wg)
	dnsServer.Serve()
}
