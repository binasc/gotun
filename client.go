//go:build linux

package main

import (
	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"gopkg.in/ini.v1"
	"net"
)

type Context struct {
	global      bool
	skippedIp   AddressSet
	remoteAddr  net.IP
	localAddr   net.IP
	phantomAddr net.IP
	chinaIPList ChinaIPList
}

func startClient(device Tun, common, client *ini.Section) {
	tunnel, err := NewClientTunnel(common, client)
	if err != nil {
		Error.Printf("Failed to create client tunnel: %v\n", err)
		return
	}
	ctx := &Context{
		global:      client.Key("global").MustBool(false),
		skippedIp:   NewAddressSet(client.Key("skipped_addresses").String()),
		remoteAddr:  net.ParseIP(client.Key("remote_addr").String()),
		localAddr:   net.ParseIP(client.Key("local_addr").String()),
		phantomAddr: net.ParseIP(client.Key("phantom_addr").String()),
		chinaIPList: NewChinaIPList("china_ip_list.txt"),
	}
	device.SetHandler(func(_ Tun, data []byte) { ctx.cliDeviceReceived(device, tunnel, data) })
	tunnel.SetHandler(func(_ Tunnel, data []byte) { device.Send(data) })
}

// Explicit exclusions take precedence over the remote endpoint and global mode.
func (ctx *Context) isViaTunnel(packet gopacket.Packet) bool {
	layer := packet.Layer(layers.LayerTypeIPv4)
	if layer == nil {
		return false
	}
	dst := layer.(*layers.IPv4).DstIP
	if ctx.skippedIp.Test(dst) {
		return false
	}
	if dst.Equal(ctx.remoteAddr) {
		return true
	}
	if dst.IsPrivate() || !dst.IsGlobalUnicast() {
		return false
	}
	return ctx.global || !ctx.chinaIPList.TestIP(dst)
}

// Serialize only the headers we change; application bytes must remain opaque.
func updateChecksum(packet gopacket.Packet) []byte {
	network := packet.NetworkLayer()
	header, ok := network.(gopacket.SerializableLayer)
	if !ok {
		return packet.Data()
	}
	parts := []gopacket.SerializableLayer{header}
	payload := network.LayerPayload()
	var err error
	if transport := packet.TransportLayer(); transport != nil {
		serializable, ok := transport.(gopacket.SerializableLayer)
		if !ok {
			return packet.Data()
		}
		switch layer := transport.(type) {
		case *layers.TCP:
			err = layer.SetNetworkLayerForChecksum(network)
		case *layers.UDP:
			err = layer.SetNetworkLayerForChecksum(network)
		}
		parts = append(parts, serializable)
		payload = transport.LayerPayload()
	} else if layer := packet.Layer(layers.LayerTypeICMPv6); layer != nil {
		icmp := layer.(*layers.ICMPv6)
		err = icmp.SetNetworkLayerForChecksum(network)
		parts = append(parts, icmp)
		payload = icmp.LayerPayload()
	}
	if err != nil {
		Error.Printf("checksum setup failed: %v\n", err)
		return packet.Data()
	}
	parts = append(parts, gopacket.Payload(payload))
	buffer := gopacket.NewSerializeBuffer()
	if err := gopacket.SerializeLayers(buffer, gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}, parts...); err != nil {
		Error.Printf("checksum serialization failed: %v\n", err)
		return packet.Data()
	}
	return buffer.Bytes()
}

func (ctx *Context) tryChangeSrc(packet gopacket.Packet) bool {
	layer := packet.Layer(layers.LayerTypeIPv4)
	if layer != nil && layer.(*layers.IPv4).Version == 4 {
		if layer.(*layers.IPv4).SrcIP.Equal(ctx.localAddr) {
			layer.(*layers.IPv4).SrcIP = copyIP(ctx.phantomAddr)
			return true
		}
	}
	return false
}

func (ctx *Context) tryRestoreDst(packet gopacket.Packet) bool {
	layer := packet.Layer(layers.LayerTypeIPv4)
	if layer != nil && layer.(*layers.IPv4).Version == 4 {
		if layer.(*layers.IPv4).DstIP.Equal(ctx.phantomAddr) {
			layer.(*layers.IPv4).DstIP = copyIP(ctx.localAddr)
			return true
		}
	}
	return false
}

var decodeOptions = gopacket.DecodeOptions{
	Lazy:               true,
	NoCopy:             true,
	SkipDecodeRecovery: true,
}

func (ctx *Context) cliDeviceReceived(device Tun, tunnel Tunnel, content []byte) {
	packet := gopacket.NewPacket(content, layers.LayerTypeIPv4, decodeOptions)
	if ctx.tryRestoreDst(packet) {
		device.Send(updateChecksum(packet))
	} else if ctx.isViaTunnel(packet) {
		tunnel.Send(content)
	} else if ctx.tryChangeSrc(packet) {
		device.Send(updateChecksum(packet))
	} else {
		device.Send(content)
	}
}
