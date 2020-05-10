package main

import (
	"flag"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"gopkg.in/ini.v1"
	"runtime"
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

	var device TunTap
	mode := cfg.Section("common").Key("mode").String()
	name := cfg.Section("common").Key("device").String()
	fmt.Printf("tuntap mode: %s, device: %s\n", mode, name)
	if mode == "tun" {
		device, err = StartTun(name)
	} else if mode == "tap" {
		device, err = StartTap(name)
	} else {
		fmt.Printf("Bad mode: %s\n", mode)
		return
	}
	if err != nil {
		fmt.Printf("Failed to create tun/tap device. %s\n", err)
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Println("Failed to get file watcher", err)
	}
	defer watcher.Close()

	fmt.Printf("Runtime OS: %s\n", runtime.GOOS)
	if clientMode {
		startClient(device, cfg.Section("common"), cfg.Section("client"), watcher)
	} else {
		startServer(device, cfg.Section("common"), cfg.Section("server"))
	}

	//go metrics.Log(metrics.DefaultRegistry, 5 * time.Second, log.New(os.Stderr, "metrics: ", log.Lmicroseconds))

	q := make(chan int)
	_ = <- q
	fmt.Println("bye")
}
