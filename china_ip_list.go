package main

import (
	"encoding/binary"
	"net"
	"sort"
	"strings"
)

type ChinaIPList interface {

	Add(ipMasks []string)

	TestUint32(ip uint32) bool

	TestIP(ip net.IP) bool

}

type IPMask struct {

	net uint32

	mask uint8

}

type ChinaIPListImpl struct {

	items []IPMask

}

func NewChinaIPList(loadFromFile string) ChinaIPList {
	ret := &ChinaIPListImpl{ nil }
	if loadFromFile != "" {
		ret.readIPMasks(loadFromFile)
	}
	return ret
}

func (cil *ChinaIPListImpl) readIPMasks(filename string) {
	count := 0
	ReadLine(filename, func(line string) {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#") {
			cil.add(line)
			count++
		}
	})
	cil.sort()
	Info.Printf("Load %v records from %v\n", count, filename)
}

func (cil *ChinaIPListImpl) ipToUint32(ip net.IP) uint32 {
	if len(ip) == 16 {
		return binary.BigEndian.Uint32(ip[12:16])
	} else {
		return binary.BigEndian.Uint32(ip)
	}
}

func (cil *ChinaIPListImpl) parse(raw string) *IPMask {
	_, ipNet, err := net.ParseCIDR(raw)
	if err != nil {
		Error.Printf("Bad IP/Net: %v\n", raw)
		return nil
	}
	ip := cil.ipToUint32(ipNet.IP)
	ones, _ := ipNet.Mask.Size()
	if ones > 32 {
		Error.Printf("Bad Net: %v\n", raw)
		return nil
	}
	return &IPMask{ ip, uint8(ones) }
}

func (cil *ChinaIPListImpl) add(ipMask string) {
	ipMask = strings.TrimSpace(ipMask)
	if len(ipMask) == 0 {
		return
	}
	cil.items = append(cil.items, *cil.parse(ipMask))
}

var masks = []uint32 {
	0x00000000,
	0x80000000,
	0xc0000000,
	0xe0000000,
	0xf0000000,
	0xf8000000,
	0xfc000000,
	0xfe000000,
	0xff000000,
	0xff800000,
	0xffc00000,
	0xffe00000,
	0xfff00000,
	0xfff80000,
	0xfffc0000,
	0xfffe0000,
	0xffff0000,
	0xffff8000,
	0xffffc000,
	0xffffe000,
	0xfffff000,
	0xfffff800,
	0xfffffc00,
	0xfffffe00,
	0xffffff00,
	0xffffff80,
	0xffffffc0,
	0xffffffe0,
	0xfffffff0,
	0xfffffff8,
	0xfffffffc,
	0xfffffffe,
	0xffffffff,
}

func (cil *ChinaIPListImpl) mask(ip uint32, mask uint8) uint32 {
	return ip & masks[mask]
}

func (cil *ChinaIPListImpl) uint32ToIP(ip uint32) net.IP {
	return net.IPv4(byte(ip >> 24), byte(ip >> 16 & 0xff), byte(ip >> 8 & 0xff), byte(ip & 0xff))
}

func (cil *ChinaIPListImpl) sort() {
	sort.Slice(cil.items, func(i, j int) bool {
		if cil.items[i].net < cil.items[j].net {
			return true
		} else if cil.items[i].net == cil.items[j].net {
			return cil.items[i].mask < cil.items[j].mask
		}
		return false
	})
	var lastNet uint32
	var lastMask uint8
	for i := 0; i < len(cil.items); i++ {
		if i > 0 && lastNet == cil.mask(cil.items[i].net, cil.items[i].mask) {
			Warning.Printf("%v/%v dup with previous %v/%v",
				cil.uint32ToIP(cil.items[i].net), cil.items[i].mask, cil.uint32ToIP(lastNet), lastMask)
			cil.items[i].net = lastNet
			cil.items[i].mask = lastMask
		} else {
			lastNet = cil.items[i].net
			lastMask = cil.items[i].mask
		}
	}
}

func (cil *ChinaIPListImpl) Add(ipMasks []string) {
	for _, ipMask := range ipMasks {
		cil.add(ipMask)
	}
	cil.sort()
}

func (cil *ChinaIPListImpl) find(i, j int, target uint32) int {
	if i >= j {
		return i - 1
	}
	mid := i + (j - i) / 2
	if cil.items[mid].net == target {
		return mid
	} else if target < cil.items[mid].net {
		return cil.find(i, mid, target)
	}
	return cil.find(mid + 1, j, target)
}

func (cil *ChinaIPListImpl) TestUint32(ip uint32) bool {
	nearestIdx := cil.find(0, len(cil.items), ip)
	if nearestIdx == -1 || nearestIdx >= len(cil.items) {
		return false
	}
	nearest := cil.items[nearestIdx]
	return cil.mask(ip, nearest.mask) == nearest.net
}

func (cil *ChinaIPListImpl) TestIP(ip net.IP) bool {
	return cil.TestUint32(cil.ipToUint32(ip))
}
