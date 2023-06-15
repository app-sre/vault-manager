package utils

// generic function to validate uniqueness across slice of attribute returned by f
func ValidKeys[T any](items []T, f func(T) string) bool {
	seen := make(map[string]struct{}, len(items))

	for _, item := range items {
		key := f(item)
		if _, exists := seen[key]; exists {
			return false
		}
		seen[key] = struct{}{}
	}

	return true
}
