package tool

import (
	"errors"
	"fmt"
)

func requireStringSliceArg(args map[string]interface{}, key string) ([]string, error) {
	if args == nil {
		return nil, errors.New("missing_args")
	}
	raw, ok := args[key]
	if !ok {
		return nil, fmt.Errorf("missing_%s", key)
	}
	switch v := raw.(type) {
	case []string:
		return v, nil
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("invalid_%s", key)
		}
		return out, nil
	case string:
		if v == "" {
			return nil, fmt.Errorf("invalid_%s", key)
		}
		return []string{v}, nil
	default:
		return nil, fmt.Errorf("invalid_%s", key)
	}
}

func optionalIntArg(args map[string]interface{}, key string) (int, error) {
	return optionalInt(args, key)
}
