package utils

func StringJoin(items []string, delim string) string {
	if len(items) == 0 {
		return ""
	}
	result := items[0]
	for _, item := range items[1:] {
		result += delim + item
	}
	return result
}
