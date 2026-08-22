package uistate

// This file collects small, defensive conversions from the loosely-typed
// values msgpack-rpc hands us (interface{} that is really int64, uint64,
// int, or float64 depending on how the value was encoded) into concrete Go
// types. Centralizing this avoids repeating type-switches in every event
// handler.

func toInt(v interface{}) int {
	switch n := v.(type) {
	case int64:
		return int(n)
	case uint64:
		return int(n)
	case int:
		return n
	case int32:
		return int(n)
	case uint32:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

func toString(v interface{}) string {
	s, _ := v.(string)
	return s
}

func toBool(v interface{}) bool {
	b, _ := v.(bool)
	return b
}

func toSlice(v interface{}) []interface{} {
	s, _ := v.([]interface{})
	return s
}

func toMap(v interface{}) map[string]interface{} {
	m, _ := v.(map[string]interface{})
	return m
}
