package mockup

import (
	"net"
	"reflect"
)

type Registry[L any, R any] struct {
	local  L
	remote R
}

func NewRegistry[L any, R any](local L, remote R) *Registry[L, R] {
	return &Registry[L, R]{local, remote}
}

func (r Registry[L, R]) link() {
	reflect.
		ValueOf(r.remote).
		Elem().
		FieldByName("Multiply").
		Set(reflect.ValueOf(
			func(multiplicant, multiplier float64) (product float64) {
				return multiplicant * multiplicant
			},
		))

	reflect.
		ValueOf(r.remote).
		Elem().
		FieldByName("Multiply").
		Set(reflect.ValueOf(
			func(multiplicant, multiplier float64) (product float64) {
				return multiplicant * multiplicant
			},
		))
}

func (r Registry[L, R]) Listen(local net.Listener) error {
	r.link()

	return nil
}

func (r Registry[L, R]) Connect(conn net.Conn) error {
	r.link()

	return nil
}
