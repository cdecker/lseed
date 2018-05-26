// Copyright 2016 Christian Decker. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package seed

// Various utilities to help building and serializing DNS answers. Big
// shoutout to miekg for his dns library :-)

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/adiabat/bech32"
	"github.com/btcsuite/btcd/btcec"
	"github.com/miekg/dns"
)

type DnsServer struct {
	netview    *NetworkView
	listenAddr string
	rootDomain string
}

func NewDnsServer(netview *NetworkView, listenAddr, rootDomain string) *DnsServer {
	return &DnsServer{
		netview:    netview,
		listenAddr: listenAddr,
		rootDomain: rootDomain,
	}
}

func addAResponse(n Node, name string, responses *[]dns.RR) {
	header := dns.RR_Header{
		Rrtype: dns.TypeA,
		Class:  dns.ClassINET,
		Ttl:    60,
		Name:   name,
	}

	for _, a := range n.Addresses {

		if a.IP.To4() == nil {
			continue
		}

		rr := &dns.A{
			Hdr: header,
			A:   a.IP.To4(),
		}
		*responses = append(*responses, rr)
	}

}

func addAAAAResponse(n Node, name string, responses *[]dns.RR) {
	header := dns.RR_Header{
		Rrtype: dns.TypeAAAA,
		Class:  dns.ClassINET,
		Ttl:    60,
		Name:   name,
	}
	for _, a := range n.Addresses {

		if a.IP.To4() != nil {
			continue
		}

		rr := &dns.AAAA{
			Hdr:  header,
			AAAA: a.IP.To16(),
		}
		*responses = append(*responses, rr)
	}
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
		nodeName := fmt.Sprintf("%s.%s.", encodedId, ds.rootDomain)
		rr := &dns.SRV{
			Hdr:      header,
			Priority: 10,
			Weight:   10,
			Target:   nodeName,
			Port:     n.Addresses[0].Port,
		}
		response.Answer = append(response.Answer, rr)
		//if n.Type&1 == 1 {
		//	addAAAAResponse(n, nodeName, &response.Extra)
		//} else {
		//	addAResponse(n, nodeName, &response.Extra)
		//}
	}

}

type DnsRequest struct {
	subdomain string
	qtype     uint16
	atypes    int
	realm     int
	node_id   string
}

func (ds *DnsServer) parseRequest(name string, qtype uint16) (*DnsRequest, error) {
	// Check that this is actually intended for us and not just some other domain
	if !strings.HasSuffix(strings.ToLower(name), fmt.Sprintf("%s.", ds.rootDomain)) {
		return nil, fmt.Errorf("malformed request: %s", name)
	}

	// Check that we actually like the request
	if qtype != dns.TypeA && qtype != dns.TypeAAAA && qtype != dns.TypeSRV {
		return nil, fmt.Errorf("refusing to handle query type %d (%s)", qtype, dns.TypeToString[qtype])
	}

	req := &DnsRequest{
		subdomain: name[:len(name)-len(ds.rootDomain)-1],
		qtype:     qtype,
		atypes:    6,
	}
	parts := strings.Split(req.subdomain, ".")

	for _, cond := range parts {
		if len(cond) == 0 {
			continue
		}
		k, v := cond[0], cond[1:]

		if k == 'r' {
			req.realm, _ = strconv.Atoi(v)
		} else if k == 'a' && qtype == dns.TypeSRV {
			req.atypes, _ = strconv.Atoi(v)
		} else if k == 'l' {
			_, bin, err := bech32.Decode(cond)
			if err != nil {
				return nil, fmt.Errorf("malformed bech32 pubkey")
			}

			p, err := btcec.ParsePubKey(bin, btcec.S256())
			if err != nil {
				return nil, fmt.Errorf("not a valid pubkey")
			}
			req.node_id = fmt.Sprintf("%x", p.SerializeCompressed())
		}
	}

	return req, nil
}

func (ds *DnsServer) handleLightningDns(w dns.ResponseWriter, r *dns.Msg) {

	if len(r.Question) < 1 {
		log.Errorf("empty request")
		return
	}

	req, err := ds.parseRequest(r.Question[0].Name, r.Question[0].Qtype)

	if err != nil {
		log.Errorf("error parsing request: %v", err)
		return
	}

	log.WithFields(log.Fields{
		"subdomain": req.subdomain,
		"type":      dns.TypeToString[req.qtype],
	}).Debugf("Incoming request")

	m := new(dns.Msg)
	m.SetReply(r)

	// Is this a wildcard query?
	if req.node_id == "" {
		switch req.qtype {
		case dns.TypeAAAA:
			ds.handleAAAAQuery(r, m)
			break
		case dns.TypeA:
			log.Debugf("Wildcard query")
			ds.handleAQuery(r, m)
			break
		case dns.TypeSRV:
			ds.handleSRVQuery(r, m)
		}
	} else {
		n, ok := ds.netview.nodes[req.node_id]
		if !ok {
			log.Debugf("Unable to find node with ID %s", req.node_id)
		}

		// Reply with the correct type
		if req.qtype == dns.TypeAAAA {
			addAAAAResponse(n, r.Question[0].Name, &m.Answer)
		} else if req.qtype == dns.TypeA {
			addAResponse(n, r.Question[0].Name, &m.Answer)
		}
	}

	w.WriteMsg(m)
	log.WithField("replies", len(m.Answer)).Debugf(
		"Replying with %d answers and %d extras.", len(m.Answer), len(m.Extra))
}

func (ds *DnsServer) Serve() {
	dns.HandleFunc(ds.rootDomain, ds.handleLightningDns)
	server := &dns.Server{Addr: ds.listenAddr, Net: "udp"}
	if err := server.ListenAndServe(); err != nil {
		log.Errorf("Failed to setup the udp server: %s\n", err.Error())
	}
}
