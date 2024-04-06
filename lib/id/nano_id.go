package id

import (
	crand "crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
)

var classicNanoIDAlphabet = [64]byte{
	'A', 'B', 'C', 'D', 'E',
	'F', 'G', 'H', 'I', 'J',
	'K', 'L', 'M', 'N', 'O',
	'P', 'Q', 'R', 'S', 'T',
	'U', 'V', 'W', 'X', 'Y',
	'Z', 'a', 'b', 'c', 'd',
	'e', 'f', 'g', 'h', 'i',
	'j', 'k', 'l', 'm', 'n',
	'o', 'p', 'q', 'r', 's',
	't', 'u', 'v', 'w', 'x',
	'y', 'z', '0', '1', '2',
	'3', '4', '5', '6', '7',
	'8', '9', '-', '_',
}

func rngUint32() uint32 {
	randUint32 := [4]byte{}
	if _, err := crand.Read(randUint32[:]); err != nil {
		panic(err)
	}
	if randUint32[3]&0x8 == 0x0 {
		return binary.LittleEndian.Uint32(randUint32[:])
	}
	return binary.BigEndian.Uint32(randUint32[:])
}

func shuffle(arr []byte) {
	size := len(arr)
	count := uint32(size >> 1)
	for i := uint32(0); i < count; i++ {
		j := rngUint32() % uint32(size)
		arr[i], arr[j] = arr[j], arr[i]
	}
}

func init() {
	shuffle(classicNanoIDAlphabet[:])
}

func ClassicNanoID(length int) (NanoIDGen, error) {
	if length < 2 || length > 255 {
		return nil, errors.New("invalid nano-id length")
	}

	preAllocSize := length * length * 8
	bytes := make([]byte, preAllocSize)
	if _, err := crand.Read(bytes); err != nil {
		return nil, fmt.Errorf("[nano-id] pre-allocate bytes failed, %w", err)
	}
	nanoID := make([]byte, length)
	offset := 0
	mask := byte(len(classicNanoIDAlphabet) - 1)

	var mu sync.Mutex
	return func() string {
		mu.Lock()
		defer mu.Unlock()

		if offset == preAllocSize {
			if _, err := crand.Read(bytes); /* impossible */ err != nil {
				panic(fmt.Errorf("[nano-id] pre-allocate bytes failed (run out of data), %w", err))
			}
			offset = 0
		}

		for i := 0; i < length; i++ {
			nanoID[i] = classicNanoIDAlphabet[bytes[i+offset]&mask]
		}
		offset += length
		return string(nanoID)
	}, nil
}
