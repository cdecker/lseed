// Copyright 2016 Christian Decker. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package seed

import (
	"net"
	"sync"
	"time"

	lrpc "github.com/cdecker/kugelblitz/lightningrpc"
)

const (
	// Default port for lightning nodes. A and AAAA queries only
	// return nodes that listen to this port, SRV queries can
	// actually specify a port, so they return all nodes.
	defaultPort = 9735
)

// A bitfield in which bit 0 indicates whether it is an IPv6 if set,
// and bit 1 indicates whether it uses the default port if set.
type NodeType uint8

// Local model of a node,
type Node struct {
	Id        string
	LastSeen  time.Time
	Ip        net.IP
	Port      uint16
	IpVersion uint8
	Type      NodeType
}

// The local view of the network
type NetworkView struct {
	nodesMut sync.Mutex
	nodes    map[string]Node
}

// Return a random sample matching the NodeType, or just any node if
// query is set to `0xFF`. Relies on random map-iteration ordering
// internally.
func (nv *NetworkView) RandomSample(query NodeType, count int) []Node {
	var found int
	var result []Node
	for _, n := range nv.nodes {
		if n.Type == query || query == 255 {
			result = append(result, n)
			count += 1
		}
		if found >= count {
			break
		}
	}
	return result
}

// Insert nodes into the map of known nodes. Existing nodes with the
// same Id are overwritten.
func (nv *NetworkView) AddNode(node lrpc.Node) Node {
	n := Node{
		Id:       node.Id,
		Ip:       net.ParseIP(node.Ip),
		Port:     node.Port,
		LastSeen: time.Now(),
	}

	if n.Ip.To4() == nil {
		n.Type |= 1
	}
	if n.Port == defaultPort {
		n.Type |= 1 << 1
	}

	nv.nodesMut.Lock()
	defer nv.nodesMut.Unlock()
	nv.nodes[n.Id] = n

	return n
}

func NewNetworkView() *NetworkView {
	return &NetworkView{
		nodesMut: sync.Mutex{},
		nodes:    make(map[string]Node),
	}
}
