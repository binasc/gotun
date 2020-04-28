package main

import (
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"gopkg.in/ini.v1"
	"net"
	"strings"
	"time"
)

type Context struct {
	tunName string
	global bool
	blocked DomainTrie
	blockedIp AddressQueue
	normalIp AddressQueue
	queryList *QueryList
	remoteAddr net.IP
	localAddr net.IP
	phantomAddr net.IP
	fastDNS net.IP
	cleanDNS net.IP
	localDNS net.IP
	tunTap TunTap
	tunnel Tunnel
}

func startClient(tunName string, common, client *ini.Section) {
	udp, err := NewClientTunnel(common, client)
	if err != nil {
		Error.Printf("Failed to create client tunnel: %v\n", err)
		return
	}
	tun := StartTun(tunName)

	global, err := client.Key("global").Bool()
	if err != nil {
		Warning.Printf("Bad global config, %v\n", err)
		global = false
	}

	ctx := Context{
		tun.Name(),
		global,
		NewDomainTrie("blocked.txt"),
		NewAddressQueueWithPersistence("blocked_records.txt"),
		NewAddressQueue(),
		NewQueryList(),
		net.ParseIP(client.Key("remote_addr").String()),
		net.ParseIP(client.Key("local_addr").String()),
		net.ParseIP(client.Key("phantom_addr").String()),
		net.ParseIP(client.Key("fast_dns").String()),
		net.ParseIP(client.Key("clean_dns").String()),
		net.ParseIP(client.Key("local_dns").String()),
		tun,
		udp,
	}

	tun.SetHandler(func (_ TunTap, content []byte) { ctx.cliDeviceReceived(tun, udp, content) })
	udp.SetHandler(func (_ Tunnel, content []byte) { ctx.cliTunnelReceived(tun, udp, content) })
}

func (ctx *Context) isViaTunnel(packet gopacket.Packet) (bool, bool) {
	layer := packet.Layer(layers.LayerTypeIPv4)
	if layer == nil {
		Error.Printf("unexptect layer %v\n", packet)
		return false, false
	}
	ipv4 := layer.(*layers.IPv4)
	if ipv4.Version == 6 {
		return true, false
	}
	if ipv4.DstIP.Equal(ctx.remoteAddr) {
		return true, false
	}
	if !ipv4.DstIP.IsGlobalUnicast() {
		return false, false
	}
	if ctx.global {
		return true, false
	}
	if ctx.blockedIp.TestIP(ipv4.DstIP) {
		Debug.Printf("ip: %v blocked\n", ipv4.DstIP)
		return true, false
	}

	layer = packet.Layer(layers.LayerTypeDNS)
	if layer != nil {
		dnsLayer := layer.(*layers.DNS)
		for _, q := range dnsLayer.Questions {
			if q.Type != layers.DNSTypeA {
				continue
			}
			qName := string(q.Name)
			if strings.HasSuffix(qName, ".lan.") || strings.HasSuffix(qName, ".lan") {
				Info.Printf("%v is local\n", qName)
				modified := ctx.queryList.ChangeToServer(dnsLayer.ID, packet.TransportLayer(), ipv4, ctx.localDNS)
				return false, modified
			} else if ctx.blocked.Test(qName) {
				Info.Printf("%v is blocked\n", qName)
				modified := ctx.queryList.ChangeToServer(dnsLayer.ID, packet.TransportLayer(), ipv4, ctx.cleanDNS)
				return true, modified
			} else {
				Info.Printf("%v is ok\n", qName)
			}
		}
		modified := ctx.queryList.ChangeToServer(dnsLayer.ID, packet.TransportLayer(), ipv4, ctx.fastDNS)
		return false, modified
	}
	return false, false
}

func updateChecksum(packet gopacket.Packet) []byte {
	networkLayer := packet.NetworkLayer()
	var err error
	if networkLayer != nil {
		switch networkLayer.LayerType() {
		case layers.LayerTypeIPv4:
			transportLayer := packet.TransportLayer()
			if transportLayer != nil {
				switch transportLayer.LayerType() {
				case layers.LayerTypeTCP:
					err = transportLayer.(*layers.TCP).SetNetworkLayerForChecksum(networkLayer.(*layers.IPv4))
				case layers.LayerTypeUDP:
					err = transportLayer.(*layers.UDP).SetNetworkLayerForChecksum(networkLayer.(*layers.IPv4))
				}
			}
		case layers.LayerTypeIPv6:
			icmp := packet.Layer(layers.LayerTypeICMPv6)
			if icmp != nil {
				err = icmp.(*layers.ICMPv6).SetNetworkLayerForChecksum(networkLayer)
			}
		}
	}
	if err != nil {
		Error.Printf("something error happen %v\n", err)
		return packet.Data()
	}
	options := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths: true,
	}
	newBuffer := gopacket.NewSerializeBuffer()
	err = gopacket.SerializePacket(newBuffer, options, packet)
	if err != nil {
		Error.Printf("failed to serialize packet: %v\n", err)
		return packet.Data()
	}
	return newBuffer.Bytes()
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
	Lazy: true,
	NoCopy: true,
	SkipDecodeRecovery: true,
}

func hasIPv4DNSLayer(packet gopacket.Packet, fn func(ipv4 *layers.IPv4, dns *layers.DNS) bool) (bool, bool) {
	ipv4Layer := packet.Layer(layers.LayerTypeIPv4)
	if ipv4Layer != nil && ipv4Layer.(*layers.IPv4).Version == 4 {
		dnsLayer := packet.Layer(layers.LayerTypeDNS)
		if dnsLayer != nil {
			return true, fn(ipv4Layer.(*layers.IPv4), dnsLayer.(*layers.DNS))
		}
	}
	return false, false
}

func (ctx *Context) cliDeviceReceived(t TunTap, udp Tunnel, content []byte) {
	packet := gopacket.NewPacket(content, layers.LayerTypeIPv4, decodeOptions)
	restored := ctx.tryRestoreDst(packet)
	if restored {
		// packet modified to fast dns must come from phantom address
		has, skip := hasIPv4DNSLayer(packet, func(ipv4 *layers.IPv4, dns *layers.DNS) bool {
			ctx.queryList.RestoreDnsSource(dns.ID, packet.TransportLayer(), ipv4)
			return false
		})
		if !has || !skip {
			t.Send(updateChecksum(packet))
		}
		return
	}

	viaTunnel, modified := ctx.isViaTunnel(packet)

	if viaTunnel {
		if modified {
			udp.Send(updateChecksum(packet))
		} else {
			udp.Send(content)
		}
	} else {
		changed := ctx.tryChangeSrc(packet)
		if modified || changed {
			t.Send(updateChecksum(packet))
		} else {
			t.Send(content)
		}
	}
}

func (ctx *Context) cliTunnelReceived(t TunTap, _ Tunnel, content []byte) {
	if ctx.global {
		t.Send(content)
		return
	}
	packet := gopacket.NewPacket(content, layers.LayerTypeIPv4, decodeOptions)
	has, modified := hasIPv4DNSLayer(packet, func(ipv4 *layers.IPv4, dns *layers.DNS) bool {
		for _, ans := range dns.Answers {
			if ans.Type == layers.DNSTypeA {
				ctx.blockedIp.Add(int64(ans.TTL) * time.Second.Milliseconds(), ans.IP, string(ans.Name))
			}
		}
		return ctx.queryList.RestoreDnsSource(dns.ID, packet.TransportLayer(), ipv4)
	})
	if has && modified {
		t.Send(updateChecksum(packet))
	} else {
		t.Send(content)
	}
}
