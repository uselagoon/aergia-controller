package unidler

import (
	"bufio"
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
