package biz

import "sort"

func sortedValuesFromSlice[T any](items []T, less func(a, b T) bool) []T {
	out := append([]T(nil), items...)
	sort.Slice(out, func(i, j int) bool { return less(out[i], out[j]) })
	return out
}
