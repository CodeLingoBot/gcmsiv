package gcmsiv

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

func TestToFromBytes(t *testing.T) {
	a := fieldElement([4]uint64{0x3b2c8aefd44be966, 0x2e2b34ca59fa4c88, 0, 0})

	aBytes := a.Bytes()
	if result := fieldElementFromBytes(aBytes[:]); result != a {
		t.Errorf("Converting to/from bytes does not round-trip: got %s, want %s", result, a)
	}
}

func TestFieldOps(t *testing.T) {
	a := fieldElement([4]uint64{0x3b2c8aefd44be966, 0x2e2b34ca59fa4c88, 0, 0})
	b := fieldElement([4]uint64{0xff, 0, 0, 0})

	addResult := a.add(b).String()
	if expected := "2e2b34ca59fa4c883b2c8aefd44be999"; expected != addResult {
		t.Errorf("a+b = %s, but expected %s", addResult, expected)
	}

	mulResult := a.mul(b).String()
	if expected := "e90aaa71616dbc6ef29ddce975618537"; expected != mulResult {
		t.Errorf("a⊗b = %s, but expected %s", mulResult, expected)
	}

	dotResult := a.dot(b).String()
	if expected := "94c340816b42d63aea917e1e4063e5eb"; expected != dotResult {
		t.Errorf("a•b = %s, but expected %s", dotResult, expected)
	}
}

func TestPolyval(t *testing.T) {
	var hBytes [16]byte
	hBytes[0] = 3

	input := make([]byte, 32)
	input[0] = 1
	input[16] = 0x40

	polyvalResult := fmt.Sprintf("%x", polyval(hBytes, input))
	if expected := "95000000000000000000000000283bfd"; expected != polyvalResult {
		t.Errorf("polyval(h, input) = %s, but expected %s", polyvalResult, expected)
	}
}

func TestHelloWorld(t *testing.T) {
	plaintext := []byte("Hello world")
	ad := []byte("example")
	key, _ := hex.DecodeString("4f2229294acbdf99c4584ec0e6e23638fab3a110b8ae672eba07d91ba52d6cea")
	nonce, _ := hex.DecodeString("752abad3e0afb5f434dc4310f71f3d21")

	gcmsiv, err := NewGCMSIV(key)
	if err != nil {
		t.Fatal(err)
	}

	ciphertext := gcmsiv.Seal(nil, nonce, plaintext, ad)
	const expected = "a7a62dd84fddd34a7e4d8c8e2d69ed54c0997cae05d8b2be1d963e"
	if hexCiphertext := hex.EncodeToString(ciphertext); hexCiphertext != expected {
		t.Errorf("got %s, wanted %s", hexCiphertext, expected)
	}
}

func TestAgainstVectors128(t *testing.T) {
	in, err := os.Open("output_add_info_be128.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	processTestVectors(t, doTest, in)
}

func TestAgainstVectors256(t *testing.T) {
	in, err := os.Open("output_add_info_be256.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	processTestVectors(t, doTest, in)
}

func doTest(t *testing.T, testNum int, values map[string][]byte) {
	var key []byte

	key = append(key, values["K1"]...)
	key = append(key, values["K2"]...)

	gcmsiv, err := NewGCMSIV(key)
	if err != nil {
		t.Fatal(err)
	}

	nonce := values["NONCE"]
	msg := values["MSG"]
	ad := values["AAD"]

	ciphertext := gcmsiv.Seal(nil, nonce, msg, ad)
	tag := ciphertext[len(ciphertext)-16:]
	ct := ciphertext[:len(ciphertext)-16]

	if expectedTag := values["TAG"]; !bytes.Equal(tag, expectedTag) {
		t.Errorf("#%d: got tag %x, but expected %x", testNum, tag, expectedTag)
	}

	if expectedCiphertext := values["CT"]; !bytes.Equal(ct, expectedCiphertext) {
		t.Errorf("#%d: got ciphertext %x, but expected %x", testNum, ciphertext, expectedCiphertext)
	}

	plaintext, err := gcmsiv.Open(nil, nonce, ciphertext, ad)
	if err != nil {
		t.Errorf("#%d: failed to Open ciphertext: %s", testNum, err)
	} else if !bytes.Equal(plaintext, msg) {
		t.Errorf("#%d: Open returned %x, but should be %x", testNum, plaintext, msg)
	}
}

func processTestVectors(t *testing.T, doTest func(t *testing.T, testNum int, values map[string][]byte), in io.Reader) {
	var values map[string][]byte
	var lastKey string
	var testNum int

	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		line := scanner.Text()

		if len(line) == 0 {
			lastKey = ""
			continue
		}

		if strings.HasPrefix(line, "-----") {
			lastKey = ""
			if values != nil {
				doTest(t, testNum, values)
			}

			testNum++
			values = make(map[string][]byte)
			continue
		}

		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			if len(lastKey) == 0 {
				continue
			}

			line = strings.TrimSpace(line)
			bytes, err := hex.DecodeString(line)
			if err == nil {
				values[lastKey] = append(values[lastKey], bytes...)
			}

			continue
		}

		if i := strings.LastIndexByte(line, '='); i >= 0 {
			key := strings.TrimSpace(line[:i])
			if j := strings.IndexByte(key, '='); j >= 0 {
				key = strings.TrimSpace(key[:j])
			}

			value := strings.TrimSpace(line[i+1:])
			bytes, err := hex.DecodeString(value)
			if err == nil {
				lastKey = key
				values[key] = bytes
			}

			continue
		}
	}

	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}
