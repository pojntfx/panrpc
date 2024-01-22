package utils

import (
	"errors"
	"reflect"
)

var (
	ErrPanickedWithNonErrorValue = errors.New("panicked with no error value")
)

func Call(fn reflect.Value, in []reflect.Value) (out []reflect.Value, err error) {
	defer func() {
		if e := recover(); e != nil {
			var ok bool
			err, ok = e.(error)
			if !ok {
				err = ErrPanickedWithNonErrorValue
			}
		}
	}()

	out = fn.Call(in)

	return
}
