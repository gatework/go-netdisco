package netdisco

import (
	"bytes"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"math"
	"strconv"
)

// JSONFields preserves custom-field values without float64 rounding. Netdisco
// may encode JSONB directly or as a JSON string. Nil preserves absent/null.
type JSONFields map[string]json.RawMessage

// UnmarshalJSON preserves nullable raw values and decodes string-encoded objects.
func (f *JSONFields) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		*f = nil
		return nil
	}
	if len(data) > 0 && data[0] == '"' {
		var value string
		if err := jsonv2.Unmarshal(data, &value); err != nil {
			return err
		}
		data = []byte(value)
	}
	var fields map[string]json.RawMessage
	if err := jsonv2.Unmarshal(data, &fields); err != nil {
		return fmt.Errorf("decode custom fields: %w", err)
	}
	*f = fields
	return nil
}

// Only fields declared as logical scalars receive compatibility conversion.
// Text VLANs, timestamps and speed labels retain their upstream representation.
var wireOptions = jsonv2.WithUnmarshalers(jsonv2.JoinUnmarshalers(
	jsonv2.UnmarshalFunc(func(data []byte, out *bool) error {
		if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
			*out = false
			return nil
		}
		text, err := scalarText(data)
		if err != nil {
			return err
		}
		switch text {
		case "true", "1":
			*out = true
		case "false", "0":
			*out = false
		default:
			return fmt.Errorf("invalid boolean %q", text)
		}
		return nil
	}),
	jsonv2.UnmarshalFunc(func(data []byte, out *float64) error {
		if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
			*out = 0
			return nil
		}
		text, err := scalarText(data)
		if err != nil {
			return err
		}
		value, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return fmt.Errorf("invalid number: %w", err)
		}
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return errors.New("non-finite number")
		}
		*out = value
		return nil
	}),
	jsonv2.UnmarshalFunc(func(data []byte, out *int64) error { return decodeInteger(data, out) }),
	jsonv2.UnmarshalFunc(func(data []byte, out *int) error { return decodeInteger(data, out) }),
))

func scalarText(data []byte) (string, error) {
	data = bytes.TrimSpace(data)
	if len(data) > 0 && data[0] == '"' {
		var text string
		if err := jsonv2.Unmarshal(data, &text); err != nil {
			return "", err
		}
		return text, nil
	}
	return string(data), nil
}

func decodeInteger[T ~int | ~int64](data []byte, out *T) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*out = 0
		return nil
	}
	text, err := scalarText(data)
	if err != nil {
		return err
	}
	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid integer: %w", err)
	}
	if int64(T(value)) != value {
		return errors.New("integer overflow")
	}
	*out = T(value)
	return nil
}

func decodeJSON(data []byte, out any) error { return jsonv2.Unmarshal(data, out, wireOptions) }
