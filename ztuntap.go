package main

import (
	"fmt"
	"github.com/newtools/zsocket"
	"github.com/newtools/zsocket/nettypes"
	"net"
	"os"
)

type ZTun struct {

	name string

	sendCh chan []byte

	zs *zsocket.ZSocket

	handler func (TunTap, []byte)

}

func StartZTun(tunName string) TunTap {
	ifce, err := net.InterfaceByName(tunName)
	if err != nil {
		Error.Printf("Failed to get interface of %s, %v\n", tunName, err)
		return nil
	}
	Info.Printf("interface index: %d\n", ifce.Index)
	zs, err := zsocket.NewZSocket(ifce.Index, zsocket.ENABLE_RX|zsocket.ENABLE_TX, 32768, 128, nettypes.All)
	if err != nil {
		Error.Printf("Failed to open socket for %s, %v\n", tunName, err)
		return nil
	}
	t := ZTun{ tunName, make(chan []byte, 50), zs, nil }
	go t.send()
	go func() {
		err := t.zs.Listen(t.receive)
		Error.Printf("Error when handling packet from %s, %v\n", tunName, err)
		os.Exit(1)
	}()
	return &t
}

func (t *ZTun) Send(content []byte) {
	t.sendCh <- copyBytes(content)
}

func (t *ZTun) send() {
	for  {
		toSend := <- t.sendCh
		n, err := t.zs.WriteToBuffer(toSend, uint16(len(toSend)))
		if err != nil {
			Error.Printf("%s failed to send %d bytes, err: %v\n", t.name, len(toSend), err)
			continue
		}
		_, err, errs := t.zs.FlushFrames()
		if err != nil {
			Error.Printf("%s failed to send %d bytes, err: %v\n", t.name, len(toSend), err)
			continue
		}
		if len(errs) > 0 {
			Error.Printf("%s failed to send %d bytes, err: %v\n", t.name, len(toSend), err)
			continue
		}
		Debug.Printf("sent to %v %d bytes\n", t.name, n)
	}
}

func (t *ZTun) SetHandler(handler func(TunTap, [] byte)) {
	t.handler = handler
}

func (t *ZTun) receive(f *nettypes.Frame, frameLen, capturedLen uint16) {
	fmt.Printf("FRAME: %s\n", f.String(capturedLen, 0))
}

func (t *ZTun) Name() string {
	return t.name
}
