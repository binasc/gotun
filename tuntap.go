package main

import (
	"github.com/songgao/water"
	"os"
)

type TunTap interface {

	Send([] byte)

	SetHandler(func (TunTap, []byte))

	Name() string

}

type TunTapImpl struct {

	sendCh chan []byte

	device *water.Interface

	handler func (TunTap, []byte)

}

func StartTun(tunName string) (TunTap, error) {
	return startTunTap(water.TUN, tunName)
}

func StartTap(tapName string) (TunTap, error) {
	return startTunTap(water.TAP, tapName)
}

func startTunTap(deviceType water.DeviceType, name string) (TunTap, error) {
	tun, err := water.New(water.Config{
		DeviceType: deviceType,
		PlatformSpecificParams: PlatformSpecificParams(name),
	})
	if err != nil {
		return nil, err
	}
	Info.Printf("tun device %s created\n", tun.Name())
	t := TunTapImpl{make(chan []byte, 50), tun, nil }
	go t.send()
	go t.receive()
	return &t, nil
}

func (t *TunTapImpl) Send(content []byte) {
	t.sendCh <- copyBytes(content)
}

func (t *TunTapImpl) send() {
	for  {
		toSend := <- t.sendCh
		n, err := t.device.Write(toSend)
		if err != nil {
			Error.Printf("%s failed to send %d bytes, err: %v\n", t.Name(), len(toSend), err)
			continue
		}
		Debug.Printf("sent to %v %d bytes\n", t.Name(), n)
	}
}

func (t *TunTapImpl) SetHandler(handler func(TunTap, [] byte)) {
	t.handler = handler
}

func (t *TunTapImpl) receive() {
	buf := make([]byte, 1500)
	for {
		n, err := t.device.Read(buf)
		if err != nil {
			Error.Println("error: read:", err)
			os.Exit(1)
		}

		Debug.Printf("received %v bytes from %s\n", n, t.Name())
		if t.handler == nil {
			Warning.Printf("no handler set, skip %d bytes", n)
		} else {
			t.handler(t, buf[:n])
		}
	}
}

func (t *TunTapImpl) Name() string {
	return t.device.Name()
}
