package main

import (
	"gopkg.in/ini.v1"
)

func startServer(device TunTap, common, server *ini.Section) {
	tunnel, err := NewServerTunnel(common, server)
	if err != nil {
		Error.Printf("Failed to start server tunnel: %v\n", err)
		return
	}
	device.SetHandler(func (_ TunTap, content []byte) { svrDeviceReceived(device, tunnel, content) })
	tunnel.SetHandler(func (_ Tunnel, content []byte) { svrTunnelReceived(device, tunnel, content) })

	//f, err := os.Create("profiling")
	//if err != nil {
	//	log.Fatalf("%v\n", err)
	//}
	//pprof.StartCPUProfile(f)
	//c := make(chan os.Signal)
	//signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	//go func() {
	//	<-c
	//	pprof.StopCPUProfile()
	//	os.Exit(0)
	//}()
}

func svrDeviceReceived(_ TunTap, tunnel Tunnel, content []byte) {
	tunnel.Send(content)
}

func svrTunnelReceived(device TunTap, _ Tunnel, content []byte) {
	device.Send(content)
}