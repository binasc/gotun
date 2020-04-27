package main

import (
	"golang.org/x/net/ipv4"
	"net"
	"strconv"
	"sync/atomic"
	"syscall"
	"unsafe"
)

var rawTxLength = 64
var rawRxLength = 64

type RawTunnelImpl struct {
	protocol uint8
	sendCh chan []byte
	handler func (Tunnel, []byte)
	conn *ipv4.PacketConn
	destination *net.IPAddr
	preConnected bool
}

func newIPAddr() *net.IPAddr {
	return &net.IPAddr{ IP: net.ParseIP("::") }
}

func dupIPAddr(dst, src *net.IPAddr) {
	copy(dst.IP, src.IP.To16())
	dst.Zone = src.Zone
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

	sendCh := make(chan []byte, rawTxLength)

	destination := connect
	if destination == nil {
		destination = newIPAddr()
	}

	tunnel := RawTunnelImpl{
		protocol, sendCh, nil, ipv4.NewPacketConn(conn), destination, connect != nil,
	}
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
	messages := make([]ipv4.Message, rawTxLength)
	for i := 0; i < len(messages); i++ {
		messages[i].Buffers = [][]byte { nil }
		if !t.preConnected {
			messages[i].Addr = t.destination
		}
	}

	for  {
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

		if t.destination.IP.IsUnspecified() {
			Warning.Printf("No destination, skip %v bytes\n", bytes)
			continue
		}

		msgSent := 0
		for msgSent < count {
			n, err := t.conn.WriteBatch(messages[msgSent:count], 0)
			if err != nil {
				Error.Printf("Failed to send to %v, err: %v\n", t.destination, err)

				conn, err := net.DialIP("ip4:" + strconv.Itoa(int(t.protocol)), nil, t.destination)
				if err != nil {
					Error.Printf("Failed to re-dial to %v, err: %v\n", t.destination, err)
					break
				}
				n = 0
				old := atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&t.conn)), unsafe.Pointer(ipv4.NewPacketConn(conn)))
				err = (*ipv4.PacketConn)(old).Close()
				if err != nil {
					Error.Printf("Failed to close old connection to %v, err: %v\n", t.destination, err)
				}
			}
			msgSent += n
		}
		Debug.Printf("sent to %v %d bytes\n", t.destination, bytes)
	}
}

func (t *RawTunnelImpl) receive() {
	messages := make([]ipv4.Message, rawRxLength)
	for i := 0; i < len(messages); i++ {
		messages[i].Buffers = [][]byte { make([]byte, 2048) }
		messages[i].N = len(messages[i].Buffers[0])
	}
	for {
		n, err := t.conn.ReadBatch(messages[:], syscall.MSG_WAITFORONE)
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
			remoteAddr := msg.Addr.(*net.IPAddr)
			if !equalIPAddr(remoteAddr, t.destination) {
				if t.preConnected {
					Error.Printf("cannot change destination from %v to %v\n", t.destination, remoteAddr)
					break
				} else {
					Info.Printf("tunnel destination changed from %v to %v\n", t.destination, remoteAddr)
					dupIPAddr(t.destination, remoteAddr)
				}
			}
			if len(msg.Buffers) != 1 {
				Error.Printf("Bad msg Buffers size: %d, Flags: %d\n", len(msg.Buffers), msg.Flags)
				continue
			}
			received := t.restore(msg.Buffers[0][20:msg.N])
			if received != nil {
				t.handler(t, received)
			}

			Debug.Printf("received from %v %d bytes\n", remoteAddr, msg.N)
			msg.N = len(msg.Buffers[0])
		}
	}
}

