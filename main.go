package main

import (
	"flag"
	"fmt"
	"gopkg.in/ini.v1"
)

var (
	serverMode bool
	clientMode bool
	configFile string
)

func init() {
	const (
		serverModeUsage         = "server mode"
		clientModeUsage         = "client mode"
		configFileUsage         = "config file"
	)
	flag.BoolVar(&serverMode, "s", false, serverModeUsage)
	flag.BoolVar(&clientMode, "c", false, clientModeUsage)
	flag.StringVar(&configFile, "f", "", configFileUsage)
}

func main() {
	flag.Parse()

	if !serverMode && !clientMode {
		fmt.Printf("Must be in either server or client mode\n")
		return
	}

	if serverMode && clientMode {
		fmt.Printf("Can not be in server and client mode simultaneously\n")
		return
	}

	cfg, err := ini.Load(configFile)

	if err != nil {
		fmt.Printf("Bad config: %s\n", err)
		return
	}

	tunDevice := cfg.Section("common").Key("tun_device").String()
	fmt.Printf("tun: %s\n", tunDevice)

	if clientMode {
		startClient(tunDevice, cfg.Section("common"), cfg.Section("client"))
	} else {
		startServer(tunDevice, cfg.Section("common"), cfg.Section("server"))
	}


	q := make(chan int)
	_ = <- q
	fmt.Println("bye")
}
