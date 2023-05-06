package rpc

import (
	"context"
	"errors"
	"reflect"
	"sync"

	"github.com/google/uuid"
)

var (
	ErrNotAFunction = errors.New("not a function")

	ErrInvalidArgsCount = errors.New("invalid argument count")
	ErrInvalidArg       = errors.New("invalid argument")

	ErrClosureDoesNotExist = errors.New("closure does not exist")
)

type (
	callClosureType = func(ctx context.Context, closureID string, args []interface{}) (interface{}, error)
)

func createClosure(fn interface{}) (func(args ...interface{}) (interface{}, error), error) {
	functionType := reflect.ValueOf(fn).Type()

	if functionType.Kind() != reflect.Func {
		return nil, ErrNotAFunction
	}

	if functionType.NumOut() <= 0 || functionType.NumOut() > 2 {
		return nil, ErrInvalidReturn
	}

	if !functionType.Out(functionType.NumOut() - 1).Implements(errorType) {
		return nil, ErrInvalidReturn
	}

	return func(args ...interface{}) (interface{}, error) {
		if len(args) != functionType.NumIn() {
			return nil, ErrInvalidArgsCount
		}

		in := make([]reflect.Value, len(args))
		for i, arg := range args {
			if argType := reflect.TypeOf(arg); argType != functionType.In(i) {
				if argType.ConvertibleTo(functionType.In(i)) {
					in[i] = reflect.ValueOf(arg).Convert(functionType.In(i))
				} else {
					return nil, ErrInvalidArg
				}
			} else {
				in[i] = reflect.ValueOf(arg)
			}
		}

		out := reflect.ValueOf(fn).Call(in)
		if len(out) == 1 {
			if out[0].IsValid() && !out[0].IsNil() {
				return nil, out[0].Interface().(error)
			}

			return nil, nil
		}

		if out[1].IsValid() && !out[1].IsNil() {
			return out[0].Interface(), out[1].Interface().(error)
		}

		return out[0].Interface(), nil
	}, nil
}

type closureManager struct {
	closuresLock sync.Mutex
	closures     map[string]func(args ...interface{}) (interface{}, error)
}

func (m *closureManager) CallClosure(ctx context.Context, closureID string, args []interface{}) (interface{}, error) {
	m.closuresLock.Lock()
	closure, ok := m.closures[closureID]
	if !ok {
		m.closuresLock.Unlock()

		return nil, ErrClosureDoesNotExist
	}
	m.closuresLock.Unlock()

	return closure(args...)
}

func registerClosure(m *closureManager, fn interface{}) (string, func(), error) {
	cls, err := createClosure(fn)
	if err != nil {
		return "", func() {}, err
	}

	closureID := uuid.New().String()

	m.closuresLock.Lock()
	m.closures[closureID] = cls
	m.closuresLock.Unlock()

	return closureID, func() {
		m.closuresLock.Lock()
		delete(m.closures, closureID)
		m.closuresLock.Unlock()
	}, nil
}
