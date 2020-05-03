package main

import (
	"fmt"
	"golang.org/x/net/ipv4"
	"net"
)

var udpTxLength = 64
var udpRxLength = 64

type UDPTunnelImpl struct {
	sendCh       chan []byte
	handler      func (Tunnel, []byte)
	conn         *ipv4.PacketConn
	destination  *net.UDPAddr
	preConnected bool
}

func newUDPAddr() *net.UDPAddr {
	return &net.UDPAddr{ IP: net.ParseIP("::"), Port: 0 }
}

func dupUDPAddr(dst, src *net.UDPAddr) {
	copy(dst.IP, src.IP.To16())
	dst.Port = src.Port
	dst.Zone = src.Zone
}

func equalUDPAddr(l, r *net.UDPAddr) bool {
	if l == nil && r == nil {
		return true
	}
	if l == nil || r == nil {
		return false
	}
	return l.IP.Equal(r.IP) && l.Port == r.Port
}

func initUDPTunnel(listen, connect *net.UDPAddr) (Tunnel, error) {
	var conn *net.UDPConn
	var err error
	if listen == nil {
		conn, err = net.DialUDP("udp4", nil, connect)
	} else {
		conn, err = net.ListenUDP("udp4", listen)
	}
	if err != nil {
		return nil, err
	}

	err = conn.SetWriteBuffer(256 * 1024)
	if err != nil {
		return nil, err
	}

	sendCh := make(chan []byte, udpTxLength)

	destination := connect
	if destination == nil {
		destination = newUDPAddr()
	}

	tunnel := UDPTunnelImpl{
		sendCh, nil, ipv4.NewPacketConn(conn), destination, connect != nil,
	}
	go tunnel.send()
	go tunnel.receive()
	return &tunnel, nil
}

func UDPConnect(addr string, port uint16) (Tunnel, error) {
	udpAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%v", addr, port))
	if err != nil {
		return nil, err
	}
	return initUDPTunnel(nil, udpAddr)
}

func UDPListen(addr string, port uint16) (Tunnel, error) {
	udpAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%v", addr, port))
	if err != nil {
		return nil, err
	}
	return initUDPTunnel(udpAddr, nil)
}

func (t *UDPTunnelImpl) Send(content []byte) {
	t.sendCh <- t.obscure(content)
}

func (t *UDPTunnelImpl) SetHandler(handler func (Tunnel, []byte)) {
	t.handler = handler
}

func (t *UDPTunnelImpl) obscure(packet []byte) []byte {
	ret, err := obscure(1492 - 20 - 8, packet)
	if err != nil {
		Error.Printf("Error when obscure packet: %v\n", err)
		return nil
	}
	return ret
}

func (t *UDPTunnelImpl) restore(packet []byte) []byte {
	ret, err := restore(packet)
	if err != nil {
		Error.Printf("Error when restore packet: %v\n", err)
		return nil
	}
	return ret
}

func (t *UDPTunnelImpl) send() {
	messages := make([]ipv4.Message, udpTxLength)
	for i := 0; i < len(messages); i++ {
		messages[i].Buffers = [][]byte { nil }
		if !t.preConnected {
			messages[i].Addr = t.destination
		}
	}

	for {
		count := 0
		bytes := 0

		toSend := <- t.sendCh
		messages[count].Buffers[0] = toSend
		count++
		bytes += len(toSend)
	getToSendLoop:
		for {
			if count >= len(messages) {
				break
			}
			select {
			case toSend := <- t.sendCh:
				messages[count].Buffers[0] = toSend
				count++
				bytes += len(toSend)
			default:
				break getToSendLoop
			}
		}

		if t.destination.Port == 0 {
			Warning.Printf("No destination, skip %v bytes\n", bytes)
			continue
		}

		msgSent := 0
		for msgSent < count {
			n, err := t.conn.WriteBatch(messages[msgSent:count], 0)
			if err != nil {
				Error.Printf("Failed to send to %v, err: %v\n", t.destination, err)
				break
			}
			msgSent += n
		}
		Debug.Printf("sent to %v %d bytes\n", t.destination, bytes)
	}
}

func (t *UDPTunnelImpl) receive() {
	messages := make([]ipv4.Message, udpRxLength)
	for i := 0; i < len(messages); i++ {
		messages[i].Buffers = [][]byte { make([]byte, 2048) }
		messages[i].N = len(messages[i].Buffers[0])
	}
	for {
		n, err := t.conn.ReadBatch(messages[:], ReadBatchFlags)
		if err != nil {
			Error.Printf("Failed to receive, err: %v\n", err)
			continue
		}

		if t.handler == nil {
			Warning.Printf("no receive handler set, ignored %d * N bytes", n)
			continue
		}

		for i := 0; i < n; i++ {
			msg := &messages[i]
			remoteAddr := msg.Addr.(*net.UDPAddr)
			if !equalUDPAddr(remoteAddr, t.destination) {
				if t.preConnected {
					Error.Printf("cannot change destination from %v to %v\n", t.destination, remoteAddr)
					break
				} else {
					Info.Printf("tunnel destination changed from %v to %v\n", t.destination, remoteAddr)
					dupUDPAddr(t.destination, remoteAddr)
				}
			}
			if len(msg.Buffers) != 1 {
				Error.Printf("Bad msg Buffers size: %d, Flags: %d\n", len(msg.Buffers), msg.Flags)
				continue
			}
			received := t.restore(msg.Buffers[0][:msg.N])
			if received != nil {
				t.handler(t, received)
			}

			Debug.Printf("received from %v %d bytes\n", remoteAddr, msg.N)
			msg.N = len(msg.Buffers[0])
		}
	}
}
