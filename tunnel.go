package main

import (
	"errors"
	"gopkg.in/ini.v1"
)

type Tunnel interface {

	Send(content []byte)

	SetHandler(handler func (Tunnel, []byte))

}

func NewClientTunnel(common, client *ini.Section) (Tunnel, error) {
	tunnelType := common.Key("type").String()
	switch tunnelType {
	case "udp":
		port, err := common.Key("port").Uint()
		if err != nil {
			return nil, err
		}
		return UDPConnect(client.Key("vps_addr").String(), uint16(port))
	case "raw":
		protocol, err := common.Key("ip_proto").Uint()
		if err != nil {
			return nil, err
		}
		return RawConnect(client.Key("vps_addr").String(), uint8(protocol))
	default:
		return nil, errors.New("bad client type: " + tunnelType)
	}
}

func NewServerTunnel(common, server *ini.Section) (Tunnel, error) {
	tunnelType := common.Key("type").String()
	switch tunnelType {
	case "udp":
		port, err := common.Key("port").Uint()
		if err != nil {
			return nil, err
		}
		return UDPListen(server.Key("listen").String(), uint16(port))
	case "raw":
		protocol, err := common.Key("ip_proto").Uint()
		if err != nil {
			return nil, err
		}
		return RawListen(server.Key("listen").String(), uint8(protocol))
	default:
		return nil, errors.New("bad server type: " + tunnelType)
	}
}

