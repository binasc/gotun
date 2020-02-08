package main

import (
	"encoding/binary"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"net"
	"time"
)

type Query struct {
	ttl int64
	id uint16
	srcPort uint16
	srcIP net.IP
	original net.IP
	replaced net.IP
}

type QueryList struct {
	queries []Query
	queryMap map[uint64]Query
}

func NewQueryList() *QueryList {
	return &QueryList{
		make([]Query, 0, 16),
		make(map[uint64]Query),
	}
}

func toKey(ip net.IP, port, id uint16) uint64 {
	var ret uint64
	ret = uint64(binary.BigEndian.Uint32(ip.To4())) << 32
	ret += uint64(uint32(port) << 16 + uint32(id))
	return ret
}

func (ql *QueryList) expire() {
	now := time.Now().UnixNano()
	skipped := 0
	for _, query := range ql.queries {
		if query.ttl + int64(1 * time.Second) < now {
			skipped++
			delete(ql.queryMap, toKey(query.srcIP, query.srcPort, query.id))
		} else {
			break
		}
	}
	ql.queries = ql.queries[skipped:]
}

func (ql *QueryList) ChangeToServer(id uint16, transportLayer gopacket.TransportLayer, ipv4Layer *layers.IPv4, server net.IP) bool {
	ql.expire()
	if ipv4Layer.DstIP.Equal(server) {
		return false
	}

	var srcPort uint16
	switch transportLayer.LayerType() {
	case layers.LayerTypeTCP:
		srcPort = uint16(transportLayer.(*layers.TCP).SrcPort)
	case layers.LayerTypeUDP:
		srcPort = uint16(transportLayer.(*layers.UDP).SrcPort)
	default:
		return false
	}

	query := Query{
		time.Now().UnixNano(),
		id,
		srcPort,
		copyIP(ipv4Layer.SrcIP),
		copyIP(ipv4Layer.DstIP),
		copyIP(server),
	}
	ql.queries = append(ql.queries, query)

	key := toKey(query.srcIP, query.srcPort, query.id)
	ql.queryMap[key] = query

	ipv4Layer.DstIP = copyIP(server)
	return true
}

func (ql *QueryList) RestoreDnsSource(id uint16, transportLayer gopacket.TransportLayer, ipv4Layer *layers.IPv4) bool {
	var dstPort uint16
	switch transportLayer.LayerType() {
	case layers.LayerTypeTCP:
		dstPort = uint16(transportLayer.(*layers.TCP).DstPort)
	case layers.LayerTypeUDP:
		dstPort = uint16(transportLayer.(*layers.UDP).DstPort)
	default:
		return false
	}

	key := toKey(ipv4Layer.DstIP, dstPort, id)
	if query, ok := ql.queryMap[key]; ok {
		if query.replaced.Equal(ipv4Layer.SrcIP) {
			ipv4Layer.SrcIP = copyIP(query.original)
			return true
		}
	}
	return false
}
