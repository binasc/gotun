package main

import (
	"fmt"
	"golang.org/x/net/ipv4"
	"net"
	"syscall"
)

type UDPTunnelImpl struct {
	sendCh chan []byte
	handler func (Tunnel, []byte)
	conn *net.UDPConn
	pConn *ipv4.PacketConn
	destination *net.UDPAddr
	preConnected bool
}

func initUDPTunnel(listen, connect *net.UDPAddr) (Tunnel, error) {
	var conn *net.UDPConn
	//var conn net.PacketConn
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

	pConn := ipv4.NewPacketConn(conn)

	sendCh := make(chan []byte, 50)

	tunnel := UDPTunnelImpl{ sendCh, nil, conn, pConn, connect, connect != nil }
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
	t.sendCh <- copyBytes(content)
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

func sumBytes(bytes [][]byte) int {
	sum := 0
	for _, toSend := range bytes {
		sum += len(toSend)
	}
	return sum
}

func (t *UDPTunnelImpl) send() {
	toSends := make([][]byte, 0, 50)
	messages := make([]ipv4.Message, 0, 50)

	for {
		toSends = append(toSends, <- t.sendCh)
	forLoop:
		for {
			select {
			case toSend := <- t.sendCh:
				toSends = append(toSends, toSend)
			default:
				break forLoop
			}
		}

		if t.destination == nil {
			Warning.Printf("No destination, skip %v bytes\n", sumBytes(toSends))
			continue
		}
		for i := 0; i < len(toSends); i++ {
			messages = append(messages, ipv4.Message{
				Buffers: [][]byte { t.obscure(toSends[i]) },
			})
			if !t.preConnected {
				messages[len(messages)-1].Addr = t.destination
			}
		}

		n, err := t.pConn.WriteBatch(messages, 0)
		if err != nil {
			Error.Printf("Failed to send %d bytes, err: %v\n", sumBytes(toSends), err)
			continue
		}
		Debug.Printf("sent to %v %d bytes\n", t.destination, n)
		toSends = toSends[:0]
		messages = messages[:0]
	}
}

func (t *UDPTunnelImpl) receive() {
	//buffer := make([]byte, 65536)
	ms := make([]ipv4.Message, 50)
	for i := 0; i < len(ms); i++ {
		ms[i].Buffers = [][]byte { make([]byte, 4096) }
		ms[i].N = len(ms[i].Buffers[0])
	}
	for {
		n, err := t.pConn.ReadBatch(ms[:], syscall.MSG_WAITFORONE)
		if err != nil {
			Error.Printf("Failed to receive, err: %v\n", err)
			continue
		}

		//n, remoteAddr, err := t.conn.ReadFromUDP(buffer)
		//if err != nil {
		//	Error.Printf("Failed to receive, err: %v\n", err)
		//	continue
		//}
		for i := 0; i < n; i++ {
			msg := &ms[i]
			remoteAddr := msg.Addr.(*net.UDPAddr)
			if !equalUDPAddr(remoteAddr, t.destination) {
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
				if len(msg.Buffers) != 1 {
					Error.Printf("Bad msg Buffers size: %d, Flags: %d\n", len(msg.Buffers), msg.Flags)
					continue
				}
				//received := t.restore(buffer[:n])
				received := t.restore(msg.Buffers[0][:msg.N])
				if received != nil {
					t.handler(t, received)
				}
			}
			msg.N = cap(msg.Buffers[0])
		}
	}
}
