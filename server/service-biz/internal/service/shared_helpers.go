package service

import "time"

func currentTime(now func() time.Time) time.Time {
	if now != nil {
		return now()
	}
	return time.Now()
}

func generatedID(generate func() string, fallback string) string {
	if generate != nil {
		return generate()
	}
	return fallback
}

func scopedID(generate func(string) string, scope string) string {
	if generate != nil {
		return generate(scope)
	}
	return scope
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonZero(values ...int64) int64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func repositoryID[T any](repo any, fallback string, resolve func(T) string) string {
	provider, ok := repo.(T)
	if !ok {
		return fallback
	}
	return resolve(provider)
}
