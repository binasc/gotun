package main

import (
	"github.com/songgao/water"
)

func PlatformSpecificParams(tunName string) water.PlatformSpecificParams {
	return water.PlatformSpecificParams {
		Name: tunName,
		Driver: water.MacOSDriverSystem,
	}
}
