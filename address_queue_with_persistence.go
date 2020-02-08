package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type AddressQueueWithPersistenceImpl struct {

	addressQueue *AddressQueueImpl

	lock sync.Mutex

	flock sync.Mutex

	persistFile string

	lastSaved int64

}

func NewAddressQueueWithPersistence(filename string) AddressQueue {
	ret := AddressQueueWithPersistenceImpl {
		NewAddressQueue().(*AddressQueueImpl),
		sync.Mutex{},
		sync.Mutex{},
		filename,
		0,
	}

	ret.restore()

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		ret.flush()
		os.Exit(0)
	}()

	return &ret
}

func (aq *AddressQueueWithPersistenceImpl) restore() {
	n := 0
	ReadLine(aq.persistFile, func(line string) {
		line = strings.TrimSuffix(line, "\n")
		content := strings.Split(line, " ")
		if len(content) != 3 {
			Error.Printf("Bad line: %s\n", line)
			return
		}
		expiredAt, err := strconv.ParseInt(content[0], 10, 64)
		if err != nil {
			Error.Printf("Bad line of expiredAt: %s\n", line)
			return
		}
		ip := net.ParseIP(content[1])
		if ip == nil {
			Error.Printf("Bad line of IP: %s\n", line)
			return
		}
		aq.addressQueue.add(expiredAt, ip, content[2])
		n++
	})
	Info.Printf("Read %d records from %s\n", n, aq.persistFile)
}

func (aq *AddressQueueWithPersistenceImpl) persist(copied PriorityQueue) {
	go func() {
		aq.flock.Lock()
		defer aq.flock.Unlock()

		fo, err := os.Create(aq.persistFile)
		if err != nil {
			Error.Printf("Failed to persist address to %s, %v\n", aq.persistFile, err)
			return
		}
		defer func() {
			if err := fo.Close(); err != nil {
				Error.Printf("Failed to close persist file %s, %v\n", aq.persistFile, err)
			}
		}()
		w := bufio.NewWriter(fo)
		for _, record := range copied {
			bytes := fmt.Sprintf("%d %s %s\n",
				record.ttl,
				record.ip.String(),
				record.domain)
			_, err := w.WriteString(bytes)
			if err != nil {
				Error.Printf("Failed to write record. %v\n", err)
				break
			}
		}
		if err := w.Flush(); err != nil {
			Error.Printf("Failed to flush. %v\n", err)
		}
	}()
}

func (aq *AddressQueueWithPersistenceImpl) flush() {
	copied := aq.addressQueue.copy()
	if copied != nil && len(copied) > 0 {
		Info.Printf("Flushed %d records\n", len(copied))
		aq.persist(copied)
	}
}

func (aq *AddressQueueWithPersistenceImpl) Add(ttlMs int64, ip net.IP, domain string) {
	aq.addressQueue.Add(ttlMs, ip, domain)

	aq.lock.Lock()
	defer aq.lock.Unlock()

	now := time.Now().UnixNano()
	if now - aq.lastSaved > 10 * time.Second.Nanoseconds() {
		aq.lastSaved = now
		copied := aq.addressQueue.copy()
		if copied != nil && len(copied) > 0 {
			aq.persist(copied)
		}
	}
}

func (aq *AddressQueueWithPersistenceImpl) TestIP(ip net.IP) bool {
	return aq.addressQueue.TestIP(ip)
}
