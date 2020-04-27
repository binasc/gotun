package main

import (
	"encoding/binary"
	"fmt"
	"github.com/google/gopacket/layers"
	"net"
)

func copyIP(ip net.IP) net.IP {
	return copyBytes(ip)
}

func copyBytes(content []byte) []byte {
	dup := make([]byte, len(content))
	copy(dup, content)
	return dup
}

func UpdateIpv4Checksum(ipv4 *layers.IPv4) {
	bytes := ipv4.Contents

	// Clear checksum bytes
	bytes[10] = 0
	bytes[11] = 0

	// Compute checksum
	var csum uint32
	for i := 0; i < len(bytes); i += 2 {
		csum += uint32(bytes[i]) << 8
		csum += uint32(bytes[i+1])
	}
	for {
		// Break when sum is less or equals to 0xFFFF
		if csum <= 65535 {
			break
		}
		// Add carry to the sum
		csum = (csum >> 16) + uint32(uint16(csum))
	}
	// Flip all the bits
	fmt.Println("old", ipv4.Checksum)
	ipv4.Checksum =  ^uint16(csum)
	fmt.Println("new", ipv4.Checksum)
	binary.BigEndian.PutUint16(bytes[10:], ipv4.Checksum)
}
