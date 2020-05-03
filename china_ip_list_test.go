package main

import (
	"net"
	"testing"
)

func TestChinaIPListTest(t *testing.T) {

	tests := []struct { netList []string; ip string; expect bool } {
		{ []string {"192.168.1.0/24", "192.168.2.0/24"}, "192.168.1.1", true },
		{ []string {"192.168.1.0/24", "192.168.1.48/26"}, "192.168.2.1", false },
		{ []string {"203.90.8.0/21", "203.90.0.0/22"}, "203.90.8.1", true },
	}

	for _, test := range tests {
		list := NewChinaIPList("")
		list.Add(test.netList)

		result := list.TestIP(net.ParseIP(test.ip))
		if result != test.expect {
			t.Errorf("Expect test on %s is %v, but got %v", test.ip, test.expect, result)
		}
	}
}

