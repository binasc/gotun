//go:build linux

package main

import "syscall"

var ReadBatchFlags = syscall.MSG_WAITFORONE
