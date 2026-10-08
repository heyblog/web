package databasebackup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

func canonicalRow(raw json.RawMessage) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var row map[string]any
	if err := decoder.Decode(&row); err != nil || row == nil {
		return nil, ErrInvalid
	}
	return json.Marshal(row)
}

func strictDecode(raw json.RawMessage, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("%w: decode contract", ErrInvalid)
	}
	return nil
}

// scanJSON rejects duplicate keys before any database operation. Its memory is
// proportional to the largest object, rather than the number of rows.
func scanJSON(decoder *json.Decoder, depth int) error {
	if depth > 64 {
		return ErrInvalid
	}
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("%w: JSON token", ErrInvalid)
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := make(map[string]bool)
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return ErrInvalid
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return ErrInvalid
			}
			seen[name] = true
			if err := scanJSON(decoder, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := scanJSON(decoder, depth+1); err != nil {
				return err
			}
		}
	default:
		return ErrInvalid
	}
	_, err = decoder.Token()
	return err
}

func delimiter(decoder *json.Decoder, want json.Delim) error {
	token, err := decoder.Token()
	if err != nil || token != want {
		return ErrInvalid
	}
	return nil
}

func object(decoder *json.Decoder, visit func(string) error) error {
	if err := delimiter(decoder, '{'); err != nil {
		return err
	}
	seen := make(map[string]bool)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return ErrInvalid
		}
		key, ok := token.(string)
		if !ok || seen[key] {
			return ErrInvalid
		}
		seen[key] = true
		if err := visit(key); err != nil {
			return err
		}
	}
	return delimiter(decoder, '}')
}

func requireEOF(decoder *json.Decoder) error {
	if _, err := decoder.Token(); err != io.EOF {
		return ErrInvalid
	}
	return nil
}
