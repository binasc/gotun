package main

import (
	"net"
	"strconv"
)

type RawTunnelImpl struct {
	sendCh chan []byte
	handler func (Tunnel, []byte)
	conn *net.IPConn
	destination *net.IPAddr
	preConnected bool
}

func initRawTunnel(protocol uint8, listen, connect *net.IPAddr) (Tunnel, error) {
	var conn *net.IPConn
	var err error
	if listen == nil {
		conn, err = net.DialIP("ip4:" + strconv.Itoa(int(protocol)), nil, connect)
	} else {
		conn, err = net.ListenIP("ip4:" + strconv.Itoa(int(protocol)), listen)
	}
	if err != nil {
		return nil, err
	}
	err = conn.SetWriteBuffer(256 * 1024)
	if err != nil {
		return nil, err
	}
	sendCh := make(chan []byte, 50)
	tunnel := RawTunnelImpl{ sendCh, nil, conn, connect, connect != nil }
	go tunnel.send()
	go tunnel.receive()
	return &tunnel, nil
}

func RawConnect(addr string, protocol uint8) (Tunnel, error) {
	ipAddr, err := net.ResolveIPAddr("ip4", addr)
	if err != nil {
		return nil, err
	}
	return initRawTunnel(protocol, nil, ipAddr)
}

func RawListen(addr string, protocol uint8) (Tunnel, error) {
	ipAddr, err := net.ResolveIPAddr("ip4", addr)
	if err != nil {
		return nil, err
	}
	return initRawTunnel(protocol, ipAddr, nil)
}

func (t *RawTunnelImpl) Send(content []byte) {
	t.sendCh <- copyBytes(content)
}

func (t *RawTunnelImpl) SetHandler(handler func (Tunnel, []byte)) {
	t.handler = handler
}

func (t *RawTunnelImpl) obscure(packet []byte) []byte {
	ret, err := obscure(1492 - 20, packet)
	if err != nil {
		Error.Printf("Error when obscure packet: %v\n", err)
		return nil
	}
	return ret
}

func (t *RawTunnelImpl) restore(packet []byte) []byte {
	ret, err := restore(packet)
	if err != nil {
		Error.Printf("Error when restore packet: %v\n", err)
		return nil
	}
	return ret
}

func (t *RawTunnelImpl) send() {
	for  {
		toSend := <- t.sendCh
		if t.destination == nil {
			Warning.Printf("No destination, skip %v bytes\n", len(toSend))
			continue
		}
		var n int
		var err error
		if t.preConnected {
			n, err = t.conn.Write(t.obscure(toSend))
		} else {
			n, err = t.conn.WriteTo(t.obscure(toSend), t.destination)
		}
		if err != nil {
			Error.Printf("udp failed to send %d bytes, err: %v\n", len(toSend), err)
			continue
		}
		Debug.Printf("sent to %v %d bytes\n", t.destination, n)
	}
}

func (t *RawTunnelImpl) receive() {
	buffer := make([]byte, 65536)
	for {
		n, remoteAddr, err := t.conn.ReadFromIP(buffer)
		if err != nil {
			Error.Printf("Failed to receive, err: %v\n", err)
			continue
		}
		if !equalIPAddr(remoteAddr, t.destination) {
			if t.preConnected {
				Error.Printf("cannot change destination from %v to %v\n", t.destination, remoteAddr)
			} else {
				Info.Printf("tunnel destination changed from %v to %v\n", t.destination, remoteAddr)
				t.destination = remoteAddr
			}
		}
		Debug.Printf("received from %v %v bytes\n", remoteAddr, n)
		if t.handler == nil {
			Warning.Printf("no receive handler set, ignored %d bytes", n)
		} else {
			received := t.restore(buffer[:n])
			if received != nil {
				t.handler(t, received)
			}
		}
	}
}
