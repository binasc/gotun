package main

import (
	"container/heap"
	"encoding/binary"
	"net"
	"sync"
	"time"
)

type AddressQueue interface {

	Add(ttlMs int64, ip net.IP, domain string)

	TestIP(ip net.IP) bool

}

type AddressQueueImpl struct {
	pq             PriorityQueue
	validBefore    map[uint32]int64
	ip2DomainCount map[uint32]map[string]uint32
	lock           sync.Mutex
}

func NewAddressQueue() AddressQueue {
	ret := AddressQueueImpl{
		make(PriorityQueue, 0, 16),
		make(map[uint32]int64),
		make(map[uint32]map[string]uint32),
		sync.Mutex{},
	}
	return &ret
}

func (aq *AddressQueueImpl) expire() {
	now := time.Now().UnixNano()

	for len(aq.pq) > 0 && aq.pq[0].ttl < now {
		record := heap.Pop(&aq.pq).(*Record)

		ip := binary.BigEndian.Uint32(record.ip.To4())
		if validBefore, ok := aq.validBefore[ip]; ok && validBefore < now {
			delete(aq.validBefore, ip)
			delete(aq.ip2DomainCount[ip], "*")
		}

		domainCount := aq.ip2DomainCount[ip]
		domainCount[record.domain]--

		if domainCount[record.domain] <= 0 {
			delete(domainCount, record.domain)
		}
		if len(domainCount) == 0 {
			delete(aq.ip2DomainCount, ip)
		}
	}
}

func (aq *AddressQueueImpl) add(expiredAt int64, ip net.IP, domain string) {
	heap.Push(&aq.pq, &Record {expiredAt, domain, copyIP(ip)})

	ipVal := binary.BigEndian.Uint32(ip.To4())
	if domainCount, ok := aq.ip2DomainCount[ipVal]; ok {
		domainCount[domain]++
	} else {
		domainCount = make(map[string]uint32)
		aq.ip2DomainCount[ipVal] = domainCount
		domainCount[domain] = 1
	}
}

func (aq *AddressQueueImpl) Add(ttlMs int64, ip net.IP, domain string) {
	aq.lock.Lock()
	defer aq.lock.Unlock()

	aq.expire()

	now := time.Now().UnixNano()
	expiredAt := now + ttlMs * time.Millisecond.Nanoseconds()
	aq.add(expiredAt, ip, domain)
}

func (aq *AddressQueueImpl) visit(ip net.IP) {
	ipVal := binary.BigEndian.Uint32(ip.To4())

	domain := "*"
	if domainCount, ok := aq.ip2DomainCount[ipVal]; ok {
		domainCount[domain] = 1
		aq.validBefore[ipVal] = time.Now().UnixNano() + 300 * time.Second.Nanoseconds()
	}
}

func (aq *AddressQueueImpl) TestIP(ip net.IP) bool {
	aq.lock.Lock()
	defer aq.lock.Unlock()

	aq.visit(ip)

	aq.expire()

	key := binary.BigEndian.Uint32(ip.To4())
	_, ok := aq.ip2DomainCount[key]
	return ok
}

func (aq *AddressQueueImpl) copy() PriorityQueue {
	aq.lock.Lock()
	defer aq.lock.Unlock()

	copied := make(PriorityQueue, len(aq.pq))
	copy(copied, aq.pq)
	return copied
}
