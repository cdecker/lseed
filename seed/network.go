// Copyright 2016 Christian Decker. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package seed

import (
	"net"
	"sync"
	"time"

	"github.com/niftynei/glightning/glightning"
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

type Address struct {
	IP   net.IP
	Port uint16
}

// Local model of a node,
type Node struct {
	Id        string
	LastSeen  time.Time
	Type      NodeType
	Addresses []Address
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
	var result []Node
	for _, n := range nv.nodes {
		if n.Type&query != 0 || query == 255 {
			result = append(result, n)
		}
		if len(result) == count {
			break
		}
	}
	return result
}

// Insert nodes into the map of known nodes. Existing nodes with the
// same Id are overwritten.
func (nv *NetworkView) AddNode(node *glightning.Node) Node {
	n := Node{
		Id:       node.Id,
		LastSeen: time.Now(),
	}

	for _, a := range node.Addresses {

		if a.Type != "ipv4" && a.Type != "ipv6" {
			continue
		}

		address := Address{
			IP:   net.ParseIP(a.Addr),
			Port: uint16(a.Port),
		}

		if address.IP.To4() == nil {
			n.Type |= 1
		} else {
			n.Type |= 1 << 2
		}

		if address.Port == defaultPort {
			n.Type |= 1 << 1
		}
		n.Addresses = append(n.Addresses, address)
	}
	if len(n.Addresses) == 0 {
		return n
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
