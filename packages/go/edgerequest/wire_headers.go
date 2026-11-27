package edgerequest

import "errors"

func encodeHeaderBytes(value string) string {
	encoded := make([]rune, len(value))
	for index := range len(value) {
		encoded[index] = rune(value[index])
	}
	return string(encoded)
}

func decodeHeaderBytes(value string) (string, error) {
	decoded := make([]byte, 0, len(value))
	for _, character := range value {
		if character < 0 || character > 255 {
			return "", &Error{Kind: Platform, Cause: errors.New("invalid edge header encoding")}
		}
		decoded = append(decoded, byte(character))
	}
	return string(decoded), nil
}
