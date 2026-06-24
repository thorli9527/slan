package api

func MapPayloads[T any](items []T, mapper func(T) map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, mapper(item))
	}
	return result
}
