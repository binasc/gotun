package main

import (
	"encoding/binary"
	"errors"
	"math/rand/v2"
)

func xor(dst, src, key []byte) {
	_ = key[3]
	for i, value := range src {
		dst[i] = value ^ key[i%4]
	}
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
		padLength = rand.IntN(256)
	}
	var ret []byte
	if doPadding {
		ret = make([]byte, 8+packetLength+1+padLength)
	} else {
		ret = make([]byte, 8+packetLength)
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
		xor(ret[8:], packet, ret[0:4])
		if doPadding {
			ret[len(ret)-1] = byte(padLength)
		}
		encrypted = ret[8 : 8+packetLength]
	} else {
		xor(ret[8+1+padLength:], packet, ret[0:4])
		ret[8] = byte(padLength)
		encrypted = ret[8+1+padLength : 8+1+padLength+packetLength]
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
		if length < 9+padLength {
			return nil, errors.New("no room for padding bytes")
		}
		if high2 == 2 {
			payload = packet[9+padLength:]
		} else {
			payload = packet[8 : length-1-padLength]
		}
	} else {
		payload = packet[8:]
	}

	//sha := sha256.Sum256(payload)
	//if !bytes.Equal(sha[:4], packet[4:8]) {
	//	return nil, errors.New("bad signature of payload")
	//}

	xor(payload, payload, packet[:4])
	return payload, nil
}
