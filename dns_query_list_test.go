package main

import (
	"github.com/google/gopacket/layers"
	"net"
	"testing"
)

func TestChange(t *testing.T) {
	ipLayer := layers.IPv4{
		SrcIP:    net.IPv4(127, 0, 0, 1),
		DstIP:    net.IPv4(114, 114, 114, 114),
		Protocol: layers.IPProtocolUDP,
	}
	udpLayer := layers.UDP{
		SrcPort:   0,
		DstPort:   50000,
		Length:    0,
		Checksum:  0,
	}
	server := net.IPv4(8, 8, 8, 8)

	ql := NewQueryList()
	ql.ChangeToServer(201, &udpLayer, &ipLayer, server)

	if !ipLayer.DstIP.Equal(server) {
		t.Errorf("Expect dst ip: %v, but got: %v\n", server, ipLayer.DstIP)
	}
}

func TestRestore(t *testing.T) {
	ipLayer := layers.IPv4{
		SrcIP:    net.IPv4(127, 0, 0, 1),
		DstIP:    net.IPv4(114, 114, 114, 114),
		Protocol: layers.IPProtocolUDP,
	}
	udpRequest := layers.UDP{
		SrcPort:   0,
		DstPort:   50000,
		Length:    0,
		Checksum:  0,
	}
	server := net.IPv4(8, 8, 8, 8)

	ql := NewQueryList()
	ql.ChangeToServer(201, &udpRequest, &ipLayer, server)

	// RESPONSE
	ipLayer = layers.IPv4{
		SrcIP:    net.IPv4(8, 8, 8, 8),
		DstIP:    net.IPv4(127, 0, 0, 1),
		Protocol: layers.IPProtocolUDP,
	}
	udpResponse := layers.UDP{
		SrcPort:   50000,
		DstPort:   0,
		Length:    0,
		Checksum:  0,
	}
	ql.RestoreDnsSource(201, &udpResponse, &ipLayer)

	if !ipLayer.SrcIP.Equal(net.IPv4(114, 114, 114, 114)) {
		t.Errorf("Expect src ip: %v, but got: %v\n", net.IPv4(114, 114, 114, 114), ipLayer.SrcIP)
	}
}
