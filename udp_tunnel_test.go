package main

import (
	"bytes"
	"math/rand/v2"
	"net"
	"strconv"
	"testing"
	"time"
)

func TestObscureRestore(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 17))
	for n := 0; n < 100; n++ {
		length := rng.IntN(1400) + 1
		o := make([]byte, length)
		for i := range o {
			o[i] = byte(rng.Uint32())
		}
		tun := &UDPTunnelImpl{}
		s := tun.restore(tun.obscure(o))

		if !bytes.Equal(o, s) {
			t.Errorf("failed to obscure then restore payload of %d bytes", length)
		}
	}
}

func TestEcho(t *testing.T) {
	output := make(chan int, 10)
	stopCh := make(chan int)
	handler := func(t Tunnel, b []byte) {
		v, _ := strconv.Atoi(string(b))
		output <- v
		if v < 10 {
			t.Send([]byte(strconv.Itoa(v + 1)))
		} else {
			stopCh <- 1
		}
	}

	var err error
	t0, err := UDPListen("127.0.0.1", 0)
	if err != nil {
		t.Fatalf("Failed to listen UDP: %v", err)
	}
	t0.SetHandler(handler)
	t1, err := UDPConnect("127.0.0.1", uint16(t0.(*UDPTunnelImpl).conn.LocalAddr().(*net.UDPAddr).Port))
	if err != nil {
		t.Fatalf("Failed to connect UDP: %v", err)
	}
	t1.SetHandler(handler)

	t1.Send([]byte("1"))

	select {
	case <-stopCh:
	case <-time.After(5 * time.Second):
		t.Fatal("UDP echo timeout")
	}
	for want := 1; want <= 10; want++ {
		if got := <-output; got != want {
			t.Fatalf("echo %d: got %d", want, got)
		}
	}
}
