package main

import (
	"flag"
	"os/user"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/cdecker/kugelblitz/lightningrpc"
	"github.com/cdecker/lseed/seed"
	"github.com/roasbeef/lseed/seed"
)

var (
	listenAddr = flag.String("listen", "0.0.0.0:53", "Listen address for incoming requests.")

	lndNode = flag.String("lnd-node", "localhost:10009", "The host:port of the backing lnd node")

	rootDomain = flag.String("root-domain", "nodes.lightning.directory", "Root DNS seed domain.")

	pollInterval = flag.Int("poll-interval", 30, "Time between polls to lightningd for updates")

	debug = flag.Bool("debug", false, "Be very verbose")

	numResults = flag.Int("results", 25, "How many results shall we return to a query?")
)

var (
	lightningRpc *lightningrpc.LightningRpc

	listenAddr    = flag.String("listen", "0.0.0.0:8053", "Listen address for incoming requests.")
	rootDomain    = flag.String("root-domain", "lseed.bitcoinstats.com", "Root DNS seed domain.")
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
func poller(lrpc *lightningrpc.LightningRpc, nview *seed.NetworkView) {
	for {
		r, err := lrpc.GetNodes()

		if err != nil {
			log.Errorf("Error trying to get update from lightningd: %v", err)
		} else {
			log.Debugf("Got %d nodes from lightningd", len(r.Nodes))
			for _, n := range r.Nodes {
				if len(n.Addresses) == 0 {
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
	configure()
	lightningRpc = lightningrpc.NewLightningRpc(*lightningSock)

	nview := seed.NewNetworkView()
	dnsServer := seed.NewDnsServer(nview, *listenAddr, *rootDomain)

	go poller(lightningRpc, nview)
	dnsServer.Serve()
}
