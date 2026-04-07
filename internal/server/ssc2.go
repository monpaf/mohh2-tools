package server

import (
	"hash/crc32"
)

// crc32Step performs one step of the CRC32 calculation, returning the updated state
func crc32Step(crcState uint32, b byte) uint32 {
	return (crcState >> 8) ^ crc32.IEEETable[(byte(crcState)^b)&0xff]
}

// SSC2State represents the internal state of the custom RC4-CRC cipher
type SSC2State struct {
	SBox   [256]byte
	IndexI byte   // Traditionally 'i' in RC4
	IndexJ uint32 // Traditionally 'j' in RC4, but expanded to 32-bit CRC
}

var globalRandomState SSC2State
var isInitialized uint32

// ResetInternalState resets the global random state and init flag
func ResetInternalState() {
	isInitialized = 0
	globalRandomState = SSC2State{}
}

// CryptSSC2Init initializes the S-box (Key Scheduling Algorithm)
func CryptSSC2Init(state *SSC2State, key []byte, iterations int32) {
	if isInitialized == 0 {
		isInitialized = 1
		CryptSSC2Init(&globalRandomState, []byte("hello world"), 10)
	}

	var absIterations uint32
	if iterations < 0 {
		absIterations = uint32(-iterations)
	} else {
		absIterations = uint32(iterations)
	}

	// Initialize S-box to identity (0, 1, 2 ... 255)
	if iterations >= 0 {
		for i := range state.SBox {
			state.SBox[i] = byte(i)
		}
		state.IndexI = 0
		state.IndexJ = 0
	}

	// Scramble the S-box based on the key
	if len(key) > 0 {
		totalSteps := absIterations << 8
		j := uint32(0)
		step := uint32(0)
		keyLen := uint32(len(key))

		for totalSteps > 0 {
			idx := step & 0xff
			valI := state.SBox[idx]

			j = crc32Step(j, key[step%keyLen])
			j = crc32Step(j, valI)

			// Swap SBox[i] and SBox[j]
			state.SBox[idx] = state.SBox[j&0xff]
			state.SBox[j&0xff] = valI

			step++
			totalSteps--
		}
	}
}

// CryptSSC2Apply mutates data in place using the PRGA (Pseudo-Random Generation Algorithm)
func CryptSSC2Apply(state *SSC2State, data []byte) {
	i := state.IndexI
	j := state.IndexJ

	for k := range data {
		oldI := uint32(i)
		i++
		valI := state.SBox[i]

		j = crc32Step(j, state.SBox[oldI])
		valJ := state.SBox[j&0xff]

		// Swap SBox[i] and SBox[j]
		state.SBox[i] = valJ
		state.SBox[j&0xff] = valI

		// Generate keystream byte and XOR it with the data
		diff := valI - valJ
		data[k] ^= state.SBox[diff]
	}

	// Save state for the next byte
	state.IndexI = i
	state.IndexJ = j
}

// CryptSSC2StringEncrypt encrypts a string password to an alphanumeric printable string
func CryptSSC2StringEncrypt(out []byte, input []byte, key []byte, iterations int32) {
	state := &SSC2State{}

	// These must accumulate over the loop, not reset
	keystreamAccumulator := []byte{0}
	plainByte := []byte{0}

	CryptSSC2Init(state, key, iterations)
	CryptSSC2Init(&globalRandomState, key, -iterations)
	CryptSSC2Init(&globalRandomState, []byte("ru paranoid?"), -1)

	outLen := len(out)
	pos := 0
	inputIdx := 0
	inputNull := false

	for pos < outLen-1 {
		// 1. Fetch or pad the plaintext byte
		if inputNull {
			CryptSSC2Apply(&globalRandomState, plainByte)
			plainByte[0] = (plainByte[0] & 0x3f) + 0x20 // Restrict to printable ASCII
		} else {
			if inputIdx < len(input) {
				plainByte[0] = input[inputIdx]
				inputIdx++
				if plainByte[0] == 0 {
					inputNull = true
				}
			} else {
				plainByte[0] = 0
				inputNull = true
			}
		}

		// Restrict unprintable characters
		if plainByte[0] < 0x20 || plainByte[0] > 0x7e {
			plainByte[0] = 0x7f
		}

		// 2. Step the main cipher state
		CryptSSC2Apply(state, keystreamAccumulator)

		// 3. Combine Plaintext, Keystream, and Printable ASCII offsets
		encryptedChar := uint32(plainByte[0]) + uint32(keystreamAccumulator[0])%0x60 + 0x40
		out[pos] = byte(encryptedChar%0x60) + 0x20

		pos++
	}

	if outLen > 0 {
		out[pos] = 0 // Null terminator
	}
}

// CryptSSC2StringDecrypt decrypts an alphanumeric printable ciphertext back to the original password
func CryptSSC2StringDecrypt(ciphertext []byte, key []byte, iterations int32) string {
	state := &SSC2State{}
	keystreamAccumulator := []byte{0}

	// We only need to initialize the main state to generate the keystream.
	// We do NOT need to initialize globalRandomState because that was only used
	// by the game to generate the random garbage padding, which we are going to discard anyway
	CryptSSC2Init(state, key, iterations)

	// Ignore the trailing C-style null byte if it's in the slice
	cipherLen := len(ciphertext)
	if cipherLen > 0 && ciphertext[cipherLen-1] == 0 {
		cipherLen--
	}

	plaintext := make([]byte, cipherLen)

	for pos := 0; pos < cipherLen; pos++ {
		// 1. Step the cipher state identically to the encryption loop
		CryptSSC2Apply(state, keystreamAccumulator)

		K := uint32(keystreamAccumulator[0]) % 0x60
		C := ciphertext[pos]

		// 2. Reverse the modular ASCII math
		if C >= 0x20 && C <= 0x7e {
			cPrime := uint32(C) - 0x20
			// Add 0x60 before subtracting K to prevent negative modulo underflow
			pPrime := (cPrime + 0x60 - K) % 0x60
			plaintext[pos] = byte(pPrime + 0x20)
		} else {
			plaintext[pos] = C
		}
	}

	// 3. Strip the padding
	// The encryption function converted the original null-terminator into 0x7F
	for i, b := range plaintext {
		if b == 0x7f {
			return string(plaintext[:i]) // Truncate exactly at the end of the real password
		}
	}

	return string(plaintext)
}
