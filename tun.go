//go:build linux

package main

import (
	"github.com/songgao/water"
	"os"
	"sync/atomic"
)

type Tun interface {
	Send([]byte)

	SetHandler(func(Tun, []byte))

	Name() string
}

type TunImpl struct {
	sendCh chan []byte

	device *water.Interface

	handler atomic.Pointer[func(Tun, []byte)]
}

func StartTun(name string) (Tun, error) {
	tun, err := water.New(water.Config{
		DeviceType:             water.TUN,
		PlatformSpecificParams: tunPlatformParams(name),
	})
	if err != nil {
		return nil, err
	}
	Info.Printf("tun device %s created\n", tun.Name())
	t := TunImpl{sendCh: make(chan []byte, 50), device: tun}
	go t.send()
	go t.receive()
	return &t, nil
}

func (t *TunImpl) Send(content []byte) {
	t.sendCh <- copyBytes(content)
}

func (t *TunImpl) send() {
	for {
		toSend := <-t.sendCh
		n, err := t.device.Write(toSend)
		if err != nil {
			Error.Printf("%s failed to send %d bytes, err: %v\n", t.Name(), len(toSend), err)
			continue
		}
		Debug.Printf("sent to %v %d bytes\n", t.Name(), n)
	}
}

func (t *TunImpl) SetHandler(handler func(Tun, []byte)) {
	if handler == nil {
		t.handler.Store(nil)
		return
	}
	t.handler.Store(&handler)
}

func (t *TunImpl) receive() {
	buf := make([]byte, 1500)
	for {
		n, err := t.device.Read(buf)
		if err != nil {
			Error.Println("error: read:", err)
			os.Exit(1)
		}

		Debug.Printf("received %v bytes from %s\n", n, t.Name())
		handler := t.handler.Load()
		if handler == nil {
			Warning.Printf("no handler set, skip %d bytes", n)
		} else {
			(*handler)(t, buf[:n])
		}
	}
}

func (t *TunImpl) Name() string {
	return t.device.Name()
}
