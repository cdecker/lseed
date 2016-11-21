package main

import (
	"flag"
	"os/user"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/cdecker/kugelblitz/lightningrpc"
)

var (
	lightningRpc  *lightningrpc.LightningRpc
	listenPort    = flag.Int("port", 53, "Port to listen for incoming requests.")
	pollInterval  = flag.Int("poll-interval", 10, "Time between polls to lightningd for updates")
	lightningSock = flag.String("lightning-sock", "$HOME/.lightning/lightning-rpc", "Location of the lightning socket")
	debug         = flag.Bool("debug", false, "Be very verbose")
)

const (
	// Default port for lightning nodes. A and AAAA queries only
	// return nodes that listen to this port, SRV queries can
	// actually specify a port, so they return all nodes.
	defaultPort = 9735
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

// The local view of the network
type NetworkView struct {
	nodes    []lightningrpc.Node
	nodesMut sync.Mutex
}

// Regularly polls the lightningd node and updates the local NetworkView.
func poller(lrpc *lightningrpc.LightningRpc, net *NetworkView) {
	for {
		r := lightningrpc.GetNodesResponse{}
		err := lrpc.GetNodes(&lightningrpc.Empty{}, &r)
		if err != nil {
			log.Errorf("Error trying to get update from lightningd: %v", err)
		} else {
			log.Debugf("Got %d nodes from lightningd", len(r.Nodes))
			net.nodesMut.Lock()
			net.nodes = r.Nodes
			net.nodesMut.Unlock()
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

func main() {
	configure()
	lightningRpc = lightningrpc.NewLightningRpc(*lightningSock)

	net := NetworkView{
		nodesMut: sync.Mutex{},
	}

	go poller(lightningRpc, &net)
	time.Sleep(100 * time.Second)
}
