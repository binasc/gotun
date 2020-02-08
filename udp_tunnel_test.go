package main

import (
	"bytes"
	"container/list"
	"math/rand"
	"strconv"
	"testing"
	"time"
)

func TestObscureRestore(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	for n := 0; n < 100; n++ {
		length := rand.Intn(1400) + 1
		o := make([]byte, length)
		rand.Read(o)
		tun := &UDPTunnelImpl{}
		s := tun.restore(tun.obscure(o))

		if !bytes.Equal(o, s) {
			t.Errorf("failed to obscure then restore payload of %d bytes", length)
		}
	}
}

func TestEcho(t *testing.T) {
	output := list.New()
	stopCh := make(chan int)
	handler := func(t Tunnel, b []byte) {
		v, _ := strconv.Atoi(string(b))
		output.PushBack(v)
		if v < 10 {
			t.Send([]byte(strconv.Itoa(v+1)))
		} else {
			stopCh <- 1
		}
	}

	var err error
	t0, err := UDPListen("127.0.0.1", 11111)
	if err != nil {
		t.Errorf("Failed to listen UDP: %v", err)
	}
	t0.SetHandler(handler)
	t1, err := UDPConnect("127.0.0.1", 11111)
	if err != nil {
		t.Errorf("Failed to connect UDP: %v", err)
	}
	t1.SetHandler(handler)

	t1.Send([]byte("1"))

	_ = <- stopCh

	if output.Len() != 10 {
		t.Errorf("Want 10 elements but got %d", output.Len())
	}

	e := output.Front()
	for i := 1; i <= 10; i++ {
		if e.Value != i {
			t.Errorf("Want %d but got %d", i, e.Value)
		}
		e = e.Next()
	}
}

