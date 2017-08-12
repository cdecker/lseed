package seed

import (
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/miekg/dns"
)

type parseInput struct {
	name  string
	qtype uint16
}

var parserstestsA = []struct {
	in  parseInput
	out *DnsRequest
}{
	{parseInput{"r0.root.", dns.TypeA}, &DnsRequest{
		subdomain: "r0.",
		atypes:    6,
		realm:     0,
	}},
	{parseInput{"r0.root.", dns.TypeSRV}, &DnsRequest{
		subdomain: "r0.",
		atypes:    6,
		realm:     0,
	}},
	{parseInput{"a4.r0.root.", dns.TypeSRV}, &DnsRequest{
		subdomain: "a4.r0.",
		atypes:    4,
		realm:     0,
	}},
	{parseInput{"s.o.m.t.h.i.n.g.", dns.TypeSRV}, nil},
	{parseInput{"0.root.", dns.TypeCNAME}, nil},
	{parseInput{"root.", dns.TypeA}, &DnsRequest{
		subdomain: "",
		atypes:    6,
		realm:     0,
	}},
}

func TestParseRequest(t *testing.T) {
	ds := &DnsServer{
		rootDomain: "root",
	}
	for _, tt := range parserstestsA {

		// Clone some details we are copying anyway
		if tt.out != nil {
			tt.out.qtype = tt.in.qtype
		}

		req, err := ds.parseRequest(tt.in.name, tt.in.qtype)

		if err != nil && tt.out != nil {
			t.Errorf("unexpected error %q => %q, want %q, %v", tt.in, req, tt.out, err)
		} else if !reflect.DeepEqual(req, tt.out) {
			spew.Dump(req)
			spew.Dump(tt.out)
			t.Errorf("parser error %q => %#v, want %#v", tt.in, req, tt.out)
		}
	}
}
