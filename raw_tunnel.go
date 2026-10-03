package main

import (
	"golang.org/x/net/ipv4"
	"net"
	"strconv"
	"sync/atomic"
)

var rawTxLength = 64
var rawRxLength = 64

type RawTunnelImpl struct {
	protocol     uint8
	sendCh       chan []byte
	handler      atomic.Pointer[func(Tunnel, []byte)]
	conn         atomic.Pointer[ipv4.PacketConn]
	destination  atomic.Pointer[net.IPAddr]
	preConnected bool
}

func newIPAddr() *net.IPAddr {
	return &net.IPAddr{IP: net.ParseIP("::")}
}

func copyIPAddr(src *net.IPAddr) *net.IPAddr {
	cloned := *src
	cloned.IP = copyIP(src.IP)
	return &cloned
}

func equalIPAddr(l, r *net.IPAddr) bool {
	if l == nil && r == nil {
		return true
	}
	if l == nil || r == nil {
		return false
	}
	return l.IP.Equal(r.IP)
}

func initRawTunnel(protocol uint8, listen, connect *net.IPAddr) (Tunnel, error) {
	var conn *net.IPConn
	var err error
	if listen == nil {
		conn, err = net.DialIP("ip4:"+strconv.Itoa(int(protocol)), nil, connect)
	} else {
		conn, err = net.ListenIP("ip4:"+strconv.Itoa(int(protocol)), listen)
	}
	if err != nil {
		return nil, err
	}

	err = conn.SetWriteBuffer(256 * 1024)
	if err != nil {
		return nil, err
	}

	sendCh := make(chan []byte, rawTxLength)

	destination := connect
	if destination == nil {
		destination = newIPAddr()
	}

	tunnel := RawTunnelImpl{
		protocol: protocol, sendCh: sendCh, preConnected: connect != nil,
	}
	tunnel.destination.Store(copyIPAddr(destination))
	tunnel.conn.Store(ipv4.NewPacketConn(conn))
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
	t.sendCh <- t.obscure(content)
}

func (t *RawTunnelImpl) SetHandler(handler func(Tunnel, []byte)) {
	t.handler.Store(&handler)
}

func (t *RawTunnelImpl) obscure(packet []byte) []byte {
	ret, err := obscure(1492-20, packet)
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
	messages := make([]ipv4.Message, rawTxLength)
	for i := 0; i < len(messages); i++ {
		messages[i].Buffers = [][]byte{nil}
	}

	for {
		count := 0
		bytes := 0

		toSend := <-t.sendCh
		messages[count].Buffers[0] = toSend
		count++
		bytes += len(toSend)
	getToSendLoop:
		for {
			if count >= len(messages) {
				break
			}
			select {
			case toSend := <-t.sendCh:
				messages[count].Buffers[0] = toSend
				count++
				bytes += len(toSend)
			default:
				break getToSendLoop
			}
		}

		destination := t.destination.Load()
		if destination.IP.IsUnspecified() {
			Warning.Printf("No destination, skip %v bytes\n", bytes)
			continue
		}

		if !t.preConnected {
			for i := 0; i < count; i++ {
				messages[i].Addr = destination
			}
		}
		msgSent := 0
		for msgSent < count {
			n, err := t.conn.Load().WriteBatch(messages[msgSent:count], 0)
			if err != nil {
				Error.Printf("Failed to send to %v, err: %v\n", destination, err)

				conn, err := net.DialIP("ip4:"+strconv.Itoa(int(t.protocol)), nil, destination)
				if err != nil {
					Error.Printf("Failed to re-dial to %v, err: %v\n", destination, err)
					break
				}
				n = 0
				old := t.conn.Swap(ipv4.NewPacketConn(conn))
				err = old.Close()
				if err != nil {
					Error.Printf("Failed to close old connection to %v, err: %v\n", destination, err)
				}
			}
			msgSent += n
		}
		Debug.Printf("sent to %v %d bytes\n", destination, bytes)
	}
}

func (t *RawTunnelImpl) receive() {
	messages := make([]ipv4.Message, rawRxLength)
	for i := 0; i < len(messages); i++ {
		messages[i].Buffers = [][]byte{make([]byte, 2048)}
		messages[i].N = len(messages[i].Buffers[0])
	}
	for {
		n, err := t.conn.Load().ReadBatch(messages[:], ReadBatchFlags)
		if err != nil {
			Error.Printf("Failed to receive, err: %v\n", err)
			continue
		}

		handler := t.handler.Load()
		if handler == nil {
			Warning.Printf("no receive handler set, ignored %d * N bytes", n)
			continue
		}

		for i := 0; i < n; i++ {
			msg := &messages[i]
			remoteAddr := msg.Addr.(*net.IPAddr)
			destination := t.destination.Load()
			if !equalIPAddr(remoteAddr, destination) {
				if t.preConnected {
					Error.Printf("cannot change destination from %v to %v\n", destination, remoteAddr)
					break
				} else {
					Info.Printf("tunnel destination changed from %v to %v\n", destination, remoteAddr)
					t.destination.Store(copyIPAddr(remoteAddr))
				}
			}
			if len(msg.Buffers) != 1 {
				Error.Printf("Bad msg Buffers size: %d, Flags: %d\n", len(msg.Buffers), msg.Flags)
				continue
			}
			received := t.restore(msg.Buffers[0][20:msg.N])
			if received != nil {
				(*handler)(t, received)
			}

			Debug.Printf("received from %v %d bytes\n", remoteAddr, msg.N)
			msg.N = len(msg.Buffers[0])
		}
	}
}
