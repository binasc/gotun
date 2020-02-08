package main

import (
	"encoding/binary"
	"errors"
	"github.com/lukechampine/fastxor"
	"math/rand"
	"unsafe"
)

func toUint32Array(bytes []byte) []uint32 {
	return *(*[]uint32)(unsafe.Pointer(&bytes))
}

func xor(dst, src, key []byte) {
	// fast fail
	_ = key[3]
	current := 0
	bound := len(src) / 4 * 4
	keyVal := toUint32Array(key)[0]
	for ; current < bound; current += 4 {
		toUint32Array(dst[current:current+4])[0] =
			toUint32Array(src[current:current+4])[0] ^ keyVal
	}
	for i := 0; current < len(src); current, i = current+1, i+1 {
		dst[current] = src[current] ^ key[i]
	}
}

func fastXor(dst, src, key []byte) {
	// fast fail
	_ = key[3]
	current := 0
	bound := len(src) / 16 * 16
	if bound > 0 {
		extKey := make([]byte, 16)
		for i := 0; i < 16; i += 4 {
			copy(extKey[i:i+4], key)
		}
		for ; current < bound; current += 16 {
			fastxor.Block(dst[current:current+16], src[current:current+16], extKey)
		}
	}
	xor(dst[current:], src[current:], key)
}

func obscure(mss int, packet []byte) ([]byte, error) {
	packetLength := len(packet)
	remainLength := mss - 8 - len(packet)
	if remainLength < 0 {
		return nil, errors.New("max segment size is smaller than packet size")
	}
	doPadding := remainLength >= 256
	padLength := 0
	if doPadding {
		padLength = rand.Intn(256)
	}
	var ret []byte
	if doPadding {
		ret = make([]byte, 8 + packetLength + 1 + padLength)
	} else {
		ret = make([]byte, 8 + packetLength)
	}

	key := rand.Uint32()
	binary.BigEndian.PutUint32(ret, key)

	placePadFirst := (ret[0] & 0x80) != 0
	if doPadding != placePadFirst {
		ret[0] = ret[0] | 0x40
	} else {
		ret[0] = ret[0] & 0xBF
	}

	var encrypted []byte
	if !doPadding || (doPadding && !placePadFirst) {
		fastXor(ret[8:], packet, ret[0:4])
		if doPadding {
			ret[len(ret)-1] = byte(padLength)
		}
		encrypted = ret[8:8+packetLength]
	} else {
		fastXor(ret[8+1+padLength:], packet, ret[0:4])
		ret[8] = byte(padLength)
		encrypted = ret[8+1+padLength:8+1+padLength+packetLength]
	}

	_ = encrypted
	//sha := sha256.Sum256(encrypted)
	//toUint32Array(ret[4:])[0] = toUint32Array(sha[:])[0]

	return ret, nil
}

func restore(packet []byte) ([]byte, error) {
	length := len(packet)
	if length < 8 {
		return nil, errors.New("short packet")
	}

	high2 := packet[0] >> 6
	hasPadding := high2 == 1 || high2 == 2

	var payload []byte
	if hasPadding {
		if length < 9 {
			return nil, errors.New("no room for padding length byte")
		}
		var padLength int
		if high2 == 2 {
			padLength = int(packet[8])
		} else {
			padLength = int(packet[length-1])
		}
		if length < 9 + padLength {
			return nil, errors.New("no room for padding bytes")
		}
		if high2 == 2 {
			payload = packet[9+padLength:]
		} else {
			payload = packet[8:length-1-padLength]
		}
	} else {
		payload = packet[8:]
	}

	//sha := sha256.Sum256(payload)
	//if !bytes.Equal(sha[:4], packet[4:8]) {
	//	return nil, errors.New("bad signature of payload")
	//}

	fastXor(payload, payload, packet[:4])
	return payload, nil
}
