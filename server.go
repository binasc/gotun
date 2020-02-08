package main

import (
	"gopkg.in/ini.v1"
)

func startServer(tunName string, common, server *ini.Section) {
	tunnel, err := NewServerTunnel(common, server)
	if err != nil {
		Error.Printf("Failed to start server tunnel: %v\n", err)
		return
	}
	tun := StartTun(tunName)
	tun.SetHandler(func (_ TunTap, content []byte) { svrDeviceReceived(tun, tunnel, content) })
	tunnel.SetHandler(func (_ Tunnel, content []byte) { svrTunnelReceived(tun, tunnel, content) })

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

func svrTunnelReceived(tun TunTap, _ Tunnel, content []byte) {
	tun.Send(content)
}