package filter

import (
	"github.com/sboehler/knut/lib/common/regex"
)

type Filter[T any] func(T) bool

func And[T any](fs ...Filter[T]) Filter[T] {
	return func(t T) bool {
		for _, f := range fs {
			if !f(t) {
				return false
			}
		}
		return true
	}
}

func AllowAll[T any](_ T) bool {
	return true
}

type Named interface {
	Name() string
}

func ByName[T Named](rxs regex.Regexes) Filter[T] {
	if len(rxs) == 0 {
		return AllowAll[T]
	}
	return func(t T) bool {
		return rxs.MatchString(t.Name())
	}
}

func Or[T any](fs ...Filter[T]) Filter[T] {
	return func(t T) bool {
		for _, f := range fs {
			if f(t) {
				return true
			}
		}
		return false
	}
}

func Not[T any](f Filter[T]) Filter[T] {
	return func(t T) bool {
		return !f(t)
	}
}
