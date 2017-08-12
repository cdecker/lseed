package irc

import (
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"

	"crypto/sha256"

	log "github.com/Sirupsen/logrus"
	"github.com/btcsuite/btcd/btcec"
	lrpc "github.com/cdecker/kugelblitz/lightningrpc"
	"github.com/cdecker/lseed/seed"
	irc "github.com/thoj/go-ircevent"
)

type IrcTailer struct {
	Network *seed.NetworkView
}

func verifySig(msg []string, pubKey *btcec.PublicKey, sig *btcec.Signature) bool {
	m := []byte(strings.Join(msg, " "))

	h1 := sha256.Sum256(m)
	h2 := sha256.Sum256(h1[:])
	return sig.Verify(h2[:], pubKey)

}

func (it *IrcTailer) onMsg(event *irc.Event) {
	m := event.Message()
	if !strings.Contains(event.Message(), "NODE") {
		return
	}

	splits := strings.Split(m, " ")
	if len(splits) != 5 || splits[1] != "NODE" {
		return
	}

	rawSig, e1 := hex.DecodeString(splits[0])
	sig, e2 := btcec.ParseDERSignature(rawSig, btcec.S256())
	if e1 != nil || e2 != nil {
		return
	}

	rawPubKey, e1 := hex.DecodeString(splits[2])
	pubKey, e2 := btcec.ParsePubKey(rawPubKey, btcec.S256())
	if e1 != nil || e2 != nil {
		return
	}

	if !verifySig(splits[1:], pubKey, sig) {
		fmt.Println("signature verification failed")
		return
	}

	n := lrpc.Node{
		Id: hex.EncodeToString(pubKey.SerializeCompressed()),
		Ip: splits[3],
	}
	ip := net.ParseIP(splits[3])
	p, e1 := strconv.Atoi(splits[4])
	n.Port = uint16(p)

	if ip == nil || e1 != nil || n.Port == 0 {
		return
	}

	it.Network.AddNode(n)
	log.Debugf("Added node %s @ %s:%d", n.Id, n.Ip, n.Port)
}

func (it *IrcTailer) Start() {
	log.Infoln("Connecting to LFNet")
	ircobj := irc.IRC("lseed", "lseed")
	ircobj.UseTLS = false
	ircobj.Connect("irc.lfnet.org:6667")
	ircobj.Join("#lightning-nodes")
	ircobj.AddCallback("PRIVMSG", it.onMsg)
	go ircobj.Loop()
}
