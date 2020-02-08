package main

import (
	"container/heap"
	"net"
	"testing"
	"time"
)

func TestPriorityQueueInit(t *testing.T) {
	var records PriorityQueue = []*Record {
		{ 3, "", net.IPv4(0, 0, 0, 3) },
		{ 1, "", net.IPv4(0, 0, 0, 1) },
		{ 4, "", net.IPv4(0, 0, 0, 4) },
		{ 1, "", net.IPv4(0, 0, 0, 1) },
		{ 5, "", net.IPv4(0, 0, 0, 5) },
		{ 9, "", net.IPv4(0, 0, 0, 9) },
	}
	heap.Init(&records)

	for idx, val := range []int64 {1, 1, 3, 4, 5, 9} {
		record := heap.Pop(&records).(*Record)
		if (*record).ttl != val {
			t.Errorf("Bad head sorting result, expect %d, but got %d on %dth item", val, records[idx].ttl, idx+1)
		}
	}
}

func TestPriorityQueuePush(t *testing.T) {
	var records PriorityQueue = []*Record {
		{ 3, "", net.IPv4(0, 0, 0, 3) },
		{ 1, "", net.IPv4(0, 0, 0, 1) },
		{ 4, "", net.IPv4(0, 0, 0, 4) },
		{ 1, "", net.IPv4(0, 0, 0, 1) },
		{ 5, "", net.IPv4(0, 0, 0, 5) },
		{ 9, "", net.IPv4(0, 0, 0, 9) },
	}
	heap.Init(&records)
	heap.Push(&records, &Record {2, "", net.IPv4(0, 0, 0, 2)})
	heap.Push(&records, &Record {6, "", net.IPv4(0, 0, 0, 6)})

	for idx, val := range []int64 {1, 1, 2, 3, 4, 5, 6, 9} {
		record := heap.Pop(&records).(*Record)
		if (*record).ttl != val {
			t.Errorf("Bad head sorting result, expect %d, but got %d on %dth item", val, records[idx].ttl, idx+1)
		}
	}
}

func TestAddExpire0(t *testing.T) {
	aq := NewAddressQueue()
	ip := net.IPv4(8, 8, 8, 8)
	aq.Add(100, ip, "dns.google.com")

	if !aq.TestIP(ip) {
		t.Errorf("Expect IP: %v existed", ip)
		return
	}

	time.Sleep(200 * time.Millisecond)

	if aq.TestIP(ip) {
		t.Errorf("Expect IP: %v expired", ip)
		return
	}
}

func TestAddExpire1(t *testing.T) {
	aq := NewAddressQueue()
	ip := net.IPv4(8, 8, 8, 8)
	aq.Add(100, ip, "dns.google.com")
	aq.Add(300, ip, "dns.google.com")

	if !aq.TestIP(ip) {
		t.Errorf("Expect IP: %v existed", ip)
		return
	}

	time.Sleep(200 * time.Millisecond)

	if !aq.TestIP(ip) {
		t.Errorf("Expect IP: %v existed", ip)
		return
	}
}

func TestAddExpire2(t *testing.T) {
	aq := NewAddressQueue()
	ip0 := net.IPv4(8, 8, 8, 8)
	ip1 := net.IPv4(8, 8, 4, 4)
	aq.Add(100, ip0, "dns.google.com")
	aq.Add(300, ip1, "dns.google.com")

	if !aq.TestIP(ip0) {
		t.Errorf("Expect IP: %v existed", ip0)
		return
	}

	if !aq.TestIP(ip1) {
		t.Errorf("Expect IP: %v existed", ip1)
		return
	}

	time.Sleep(200 * time.Millisecond)

	if aq.TestIP(ip0) {
		t.Errorf("Expect IP: %v expired", ip0)
		return
	}
	if !aq.TestIP(ip1) {
		t.Errorf("Expect IP: %v existed", ip1)
		return
	}
}
