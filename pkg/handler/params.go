package handler

import (
	"fmt"
	"strconv"
)

// parseStringParam extracts a required non-empty string parameter from the tool
// arguments map. Arguments arrive JSON-decoded, so string values are decoded to
// native Go strings (but we defensively accept any stringer as well).
func parseStringParam(args map[string]interface{}, key string) (string, error) {
	if args == nil {
		return "", fmt.Errorf("missing required parameter: %s", key)
	}

	v, ok := args[key]
	if !ok {
		return "", fmt.Errorf("missing required parameter: %s", key)
	}

	switch val := v.(type) {
	case string:
		if val == "" {
			return "", fmt.Errorf("parameter %s cannot be empty", key)
		}
		return val, nil
	case fmt.Stringer:
		s := val.String()
		if s == "" {
			return "", fmt.Errorf("parameter %s cannot be empty", key)
		}
		return s, nil
	default:
		return "", fmt.Errorf("parameter %s must be a string, got %T", key, v)
	}
}

// parseBoolParam extracts an optional boolean parameter. Unset or non-boolean
// values are treated as false rather than errors, so callers can treat the
// parameter as an opt-in flag.
func parseBoolParam(args map[string]interface{}, key string) (bool, error) {
	if args == nil {
		return false, nil
	}
	v, ok := args[key]
	if !ok {
		return false, nil
	}
	switch val := v.(type) {
	case bool:
		return val, nil
	case string:
		b, err := strconv.ParseBool(val)
		if err != nil {
			return false, fmt.Errorf("parameter %s must be a boolean, got %q", key, val)
		}
		return b, nil
	default:
		return false, fmt.Errorf("parameter %s must be a boolean, got %T", key, v)
	}
}

// isSQLIdentifier reports whether the given value is a safe, bare SQL identifier
// (letters, digits, underscores; not starting with a digit). Used to guard table
// / column names before embedding them in generated SQL.
func isSQLIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_':
			// ok
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			// ok
		case r >= '0' && r <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}
