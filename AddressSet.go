package main

import (
	"encoding/binary"
	"net"
	"sort"
	"strings"
)

type AddressSet interface {

	Add(ip net.IP)

	Test(ip net.IP) bool

}

type AddressSetImpl struct {

	addresses []uint32

}

func NewAddressSet(addressList string) AddressSet {
	as := &AddressSetImpl{
		make([]uint32, 0, 10),
	}
	for _, address := range strings.Split(addressList, ",") {
		as.Add(net.ParseIP(strings.TrimSpace(address)))
	}
	return as
}

func (as *AddressSetImpl) add(ip uint32) {
	as.addresses = append(as.addresses, ip)
	sort.Slice(as.addresses, func(i, j int) bool {
		return as.addresses[i] < as.addresses[j]
	})
}

func (as *AddressSetImpl) Add(ip net.IP) {
	as.add(as.ipToUint32(ip))
}

func (as *AddressSetImpl) find(i, j int, target uint32) bool {
	if i >= j {
		return false
	}
	mid := i + (j - i) / 2
	if as.addresses[mid] == target {
		return true
	} else if target < as.addresses[mid] {
		return as.find(i, mid, target)
	}
	return as.find(mid + 1, j, target)
}

func (as *AddressSetImpl) ipToUint32(ip net.IP) uint32 {
	if len(ip) == 16 {
		return binary.BigEndian.Uint32(ip[12:16])
	} else {
		return binary.BigEndian.Uint32(ip)
	}
}

func (as *AddressSetImpl) Test(ip net.IP) bool {
	return as.find(0, len(as.addresses), as.ipToUint32(ip))

}
