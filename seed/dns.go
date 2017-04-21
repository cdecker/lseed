// Copyright 2016 Christian Decker. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package seed

// Various utilities to help building and serializing DNS answers. Big
// shoutout to miekg for his dns library :-)

import (
	"encoding/hex"
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/adiabat/bech32"
	"github.com/miekg/dns"
)

type DnsServer struct {
	netview    *NetworkView
	listenAddr string
}

func NewDnsServer(netview *NetworkView, listenAddr string) *DnsServer {
	return &DnsServer{
		netview:    netview,
		listenAddr: listenAddr,
	}
}

func addAResponse(n Node, name string, responses *[]dns.RR) {
	header := dns.RR_Header{
		Rrtype: dns.TypeA,
		Class:  dns.ClassINET,
		Ttl:    60,
		Name:   name,
	}
	rr := &dns.A{
		Hdr: header,
		A:   n.Ip,
	}
	*responses = append(*responses, rr)
}

func addAAAAResponse(n Node, name string, responses *[]dns.RR) {
	header := dns.RR_Header{
		Rrtype: dns.TypeAAAA,
		Class:  dns.ClassINET,
		Ttl:    60,
		Name:   name,
	}
	rr := &dns.AAAA{
		Hdr:  header,
		AAAA: n.Ip,
	}
	*responses = append(*responses, rr)
}

func (ds *DnsServer) handleAAAAQuery(request *dns.Msg, response *dns.Msg) {
	nodes := ds.netview.RandomSample(3, 25)
	for _, n := range nodes {
		addAAAAResponse(n, request.Question[0].Name, &response.Answer)
	}
}

func (ds *DnsServer) handleAQuery(request *dns.Msg, response *dns.Msg) {
	nodes := ds.netview.RandomSample(2, 25)

	for _, n := range nodes {
		addAResponse(n, request.Question[0].Name, &response.Answer)
	}
}

// Handle incoming SRV requests.
//
// Unlike the A and AAAA requests these are a bit ambiguous, since the
// client may either be IPv4 or IPv6, so just return a mix and let the
// client figure it out.
func (ds *DnsServer) handleSRVQuery(request *dns.Msg, response *dns.Msg) {
	nodes := ds.netview.RandomSample(255, 25)

	header := dns.RR_Header{
		Name:   request.Question[0].Name,
		Rrtype: dns.TypeSRV,
		Class:  dns.ClassINET,
		Ttl:    60,
	}

	for _, n := range nodes {
		rawId, err := hex.DecodeString(n.Id)
		if err != nil {
			continue
		}
		encodedId := bech32.Encode("ln", rawId)
		nodeName := fmt.Sprintf("%s.lseed.bitcoinstats.com.", encodedId)
		rr := &dns.SRV{
			Hdr:      header,
			Priority: 10,
			Weight:   10,
			Target:   nodeName,
			Port:     n.Port,
		}
		response.Answer = append(response.Answer, rr)
		if n.Type&1 == 1 {
			addAAAAResponse(n, nodeName, &response.Extra)
		} else {
			addAResponse(n, nodeName, &response.Extra)
		}
	}

}

func (ds *DnsServer) handleLightningDns(w dns.ResponseWriter, r *dns.Msg) {

	name := r.Question[0].Name
	qtype := r.Question[0].Qtype

	log.WithFields(log.Fields{
		"subdomain": name,
		"type":      dns.TypeToString[qtype],
	}).Debugf("Incoming request")

	m := new(dns.Msg)
	m.SetReply(r)

	if name == "lseed.bitcoinstats.com." {
		switch qtype {
		case dns.TypeAAAA:
			ds.handleAAAAQuery(r, m)
			break
		case dns.TypeA:
			ds.handleAQuery(r, m)
			break
		case dns.TypeSRV:
			ds.handleSRVQuery(r, m)
		}
	} else if name == "_nodes._tcp.lseed.bitcoinstats.com." {
		ds.handleSRVQuery(r, m)
	} else {
		splits := strings.SplitN(name, ".", 2)
		if len(splits) != 2 || len(splits[0]) != 62 {
			log.Debug("Subdomain does not appear to be a valid node Id")
			return
		}

		prefix, rawId, err := bech32.Decode(splits[0])

		if err != nil || prefix != "ln" {
			log.Errorf("Unable to decode address %s, or wrong prefix %s",
				splits[0], prefix)
		}

		id := hex.EncodeToString(rawId)
		n, ok := ds.netview.nodes[id]
		if !ok {
			log.Debugf("Unable to find node with ID %s", id)
		} else {
			log.Debugf("Found node matching ID %s %#v", id, n)
		}

		// Reply with the correct type
		if qtype == dns.TypeAAAA {
			if n.Type&1 == 1 {
				addAAAAResponse(n, name, &m.Answer)
			} else {
				addAAAAResponse(n, name, &m.Extra)
			}
		} else if qtype == dns.TypeA {
			if n.Type&1 == 0 {
				addAResponse(n, name, &m.Answer)
			} else {
				addAResponse(n, name, &m.Extra)
			}
		}
	}
	w.WriteMsg(m)
	log.WithField("replies", len(m.Answer)).Debugf(
		"Replying with %d answers and %d extras.", len(m.Answer), len(m.Extra))
}

func (ds *DnsServer) Serve() {
	dns.HandleFunc("lseed.bitcoinstats.com.", ds.handleLightningDns)
	server := &dns.Server{Addr: ds.listenAddr, Net: "udp"}
	if err := server.ListenAndServe(); err != nil {
		log.Errorf("Failed to setup the udp server: %s\n", err.Error())
	}
}
