package main

import (
	"github.com/songgao/water"
)

func PlatformSpecificParams(name string) water.PlatformSpecificParams {
	return water.PlatformSpecificParams {
		Name: name,
		Persist: true,
		MultiQueue: true,
	}
}
