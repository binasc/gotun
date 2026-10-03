//go:build linux

package main

import (
	"bytes"
	"encoding/binary"
	"net"
	"sync"
	"testing"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
)

type captureTun struct{ packets [][]byte }

func (c *captureTun) Send(b []byte)              { c.packets = append(c.packets, bytes.Clone(b)) }
func (*captureTun) Name() string                 { return "test" }
func (*captureTun) SetHandler(func(Tun, []byte)) {}

type captureTunnel struct{ captureTun }

func (*captureTunnel) SetHandler(func(Tunnel, []byte)) {}

func makeUDP(t *testing.T, src, dst string) []byte {
	t.Helper()
	ip := &layers.IPv4{Version: 4, TTL: 64, Protocol: layers.IPProtocolUDP, SrcIP: net.ParseIP(src), DstIP: net.ParseIP(dst)}
	udp := &layers.UDP{SrcPort: 40000, DstPort: 53}
	if err := udp.SetNetworkLayerForChecksum(ip); err != nil {
		t.Fatal(err)
	}
	b := gopacket.NewSerializeBuffer()
	// Deliberately malformed DNS: routing and checksum updates must not decode it.
	if err := gopacket.SerializeLayers(b, gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}, ip, udp, gopacket.Payload([]byte{0xff, 0, 0x81, 0x80, 0, 1, 0})); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func TestAdaptedRoutingAndNAT(t *testing.T) {
	list := NewChinaIPList("")
	list.Add([]string{"1.0.0.0/8"})
	for _, tc := range []struct {
		dst            string
		global, tunnel bool
	}{
		{"1.2.3.4", false, false}, {"1.2.3.4", true, true},
		{"8.8.8.8", false, true}, {"9.9.9.9", true, false},
		{"10.8.0.1", false, true}, {"10.8.0.2", true, false},
		{"172.16.0.2", true, false}, {"192.168.0.2", true, false},
		{"224.0.0.1", true, false}, {"127.0.0.1", true, false},
	} {
		t.Run(tc.dst+map[bool]string{true: "-global"}[tc.global], func(t *testing.T) {
			ctx := &Context{global: tc.global, skippedIp: NewAddressSet("9.9.9.9"), remoteAddr: net.ParseIP("10.8.0.1"), localAddr: net.ParseIP("10.9.0.2"), phantomAddr: net.ParseIP("10.9.0.3"), chinaIPList: list}
			input := makeUDP(t, "10.9.0.2", tc.dst)
			device, tunnel := &captureTun{}, &captureTunnel{}
			ctx.cliDeviceReceived(device, tunnel, input)
			if tc.tunnel {
				if len(tunnel.packets) != 1 || !bytes.Equal(tunnel.packets[0], input) || len(device.packets) != 0 {
					t.Fatal("tunnel route changed packet or used wrong output")
				}
				return
			}
			if len(device.packets) != 1 || len(tunnel.packets) != 0 {
				t.Fatal("direct packet used wrong output")
			}
			out := gopacket.NewPacket(device.packets[0], layers.LayerTypeIPv4, gopacket.Default)
			ip := out.NetworkLayer().(*layers.IPv4)
			udp := out.TransportLayer().(*layers.UDP)
			if !ip.SrcIP.Equal(ctx.phantomAddr) || !bytes.Equal(udp.Payload, input[28:]) {
				t.Fatal("NAT changed payload or failed source rewrite")
			}
			if err := udp.SetNetworkLayerForChecksum(ip); err != nil {
				t.Fatal(err)
			}
			if err, result := udp.VerifyChecksum(); err != nil || !result.Valid {
				t.Fatalf("invalid UDP checksum: %v %+v", err, result)
			}
			// A reply addressed to the phantom must return to the original local address.
			reply := makeUDP(t, tc.dst, "10.9.0.3")
			ctx.cliDeviceReceived(device, tunnel, reply)
			restored := gopacket.NewPacket(device.packets[1], layers.LayerTypeIPv4, gopacket.Default).NetworkLayer().(*layers.IPv4)
			if !restored.DstIP.Equal(ctx.localAddr) {
				t.Fatal("reply destination not restored")
			}
		})
	}
}
func TestNonIPv4AndInvalidAddresses(t *testing.T) {
	for _, data := range [][]byte{nil, {0x10}, {0x45}, append([]byte{0x60}, make([]byte, 39)...)} {
		device, tunnel := &captureTun{}, &captureTunnel{}
		(&Context{}).cliDeviceReceived(device, tunnel, data)
		expected := len(data) > 0 && data[0]>>4 == 6
		if (len(tunnel.packets) == 1) != expected || len(device.packets) != 0 {
			t.Fatalf("unexpected route for %x", data)
		}
	}
	set := NewAddressSet("broken,2001:db8::1,8.8.8.8")
	set.Add(nil)
	if set.Test(nil) || set.Test(net.ParseIP("2001:db8::1")) || !set.Test(net.ParseIP("8.8.8.8")) {
		t.Fatal("invalid IPv4 address handling")
	}
}
func TestLegacyWireFormat(t *testing.T) {
	payload := []byte("legacy peer payload")
	// The legacy format repeats the four key bytes; high bits specify padding.
	for _, key := range []uint32{0x00123456, 0x40123456, 0x80123456, 0xc0123456} {
		packet := make([]byte, 8)
		binary.BigEndian.PutUint32(packet, key)
		coded := bytes.Clone(payload)
		for i := range coded {
			coded[i] ^= packet[i%4]
		}
		switch key >> 30 {
		case 1:
			packet = append(packet, coded...)
			packet = append(packet, 0, 0, 2)
		case 2:
			packet = append(packet, 2, 0, 0)
			packet = append(packet, coded...)
		default:
			packet = append(packet, coded...)
		}
		decoded, err := restore(packet)
		if err != nil || !bytes.Equal(decoded, payload) {
			t.Fatalf("legacy decode: %x %v", key, err)
		}
	}
}
func TestConcurrentHandlerPublication(t *testing.T) {
	udp, raw, tun := &UDPTunnelImpl{}, &RawTunnelImpl{}, &TunImpl{}
	var wg sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		wg.Go(func() {
			for i := 0; i < 1000; i++ {
				udp.SetHandler(func(Tunnel, []byte) {})
				raw.SetHandler(func(Tunnel, []byte) {})
				tun.SetHandler(func(Tun, []byte) {})
				if h := udp.handler.Load(); h != nil {
					(*h)(udp, nil)
				}
				if h := raw.handler.Load(); h != nil {
					(*h)(raw, nil)
				}
				if h := tun.handler.Load(); h != nil {
					(*h)(tun, nil)
				}
				udp.SetHandler(nil)
				raw.SetHandler(nil)
				tun.SetHandler(nil)
			}
		})
	}
	wg.Wait()
}
