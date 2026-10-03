//go:build linux

package main

import (
	"github.com/songgao/water"
)

func tunPlatformParams(name string) water.PlatformSpecificParams {
	return water.PlatformSpecificParams{
		Name:       name,
		Persist:    true,
		MultiQueue: true,
	}
}
