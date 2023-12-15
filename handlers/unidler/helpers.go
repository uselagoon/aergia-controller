package unidler

import (
	"bufio"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
)

func removeStatusCode(codes string, code string) *string {
	newCodes := []string{}
	for _, codeValue := range strings.Split(codes, ",") {
		if codeValue != code {
			newCodes = append(newCodes, codeValue)
		}
	}
	if len(newCodes) == 0 {
		return nil
	}
	returnCodes := strings.Join(newCodes, ",")
	return &returnCodes
}

func ReadSliceFromFile(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func hmacSign(ns string, secret []byte) []byte {
	hmac := hmac.New(sha256.New, secret)
	hmac.Write([]byte(ns))
	dataHmac := hmac.Sum(nil)
	return dataHmac
}

func hmacVerifier(verify, toverify string, secret []byte) bool {
	hmacHex, err := hex.DecodeString(toverify)
	if err != nil {
		// error verifying, return false to reject
		return false
	}
	return hmac.Equal(hmacSign(verify, secret), hmacHex)
}

func hmacSigner(ns string, secret []byte) string {
	return hex.EncodeToString(hmacSign(ns, secret))
}
