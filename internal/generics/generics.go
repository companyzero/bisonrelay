package generics

import (
	"iter"
	"maps"
)

func CopyMapValues[Map ~map[K]V, K comparable, V any](m Map) []V {
	return Collect(maps.Values(m))
}

func Collect[V any](seq iter.Seq[V]) []V {
	var result []V
	for v := range seq {
		result = append(result, v)
	}
	return result
}
