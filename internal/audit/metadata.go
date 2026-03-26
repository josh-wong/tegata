package audit

// MetadataString extracts a string value from an EventRecord metadata map.
func MetadataString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// MetadataInt64 extracts an int64 value from an EventRecord metadata map.
// JSON numbers are stored as float64, so the conversion handles that case.
func MetadataInt64(m map[string]interface{}, key string) int64 {
	if m == nil {
		return 0
	}
	v, ok := m[key]
	if !ok {
		return 0
	}
	f, ok := v.(float64)
	if !ok {
		return 0
	}
	return int64(f)
}
