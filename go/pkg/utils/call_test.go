package utils

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCall(t *testing.T) {
	tests := []struct {
		name        string
		fn          interface{}
		args        []interface{}
		expected    []interface{}
		expectedErr error
	}{
		{
			name: "simple function call",
			fn: func(x int) int {
				return x * 2
			},
			args:        []interface{}{5},
			expected:    []interface{}{10},
			expectedErr: nil,
		},
		{
			name: "multiple return values",
			fn: func(x string, y int) (string, int) {
				return x + "!", y + 1
			},
			args:        []interface{}{"hello", 42},
			expected:    []interface{}{"hello!", 43},
			expectedErr: nil,
		},
		{
			name: "panic with error",
			fn: func() {
				panic(errors.New("test panic"))
			},
			args:        []interface{}{},
			expected:    nil,
			expectedErr: errors.New("test panic"),
		},
		{
			name: "panic with non-error value",
			fn: func() {
				panic("string panic")
			},
			args:        []interface{}{},
			expected:    nil,
			expectedErr: ErrPanickedWithNonErrorValue,
		},
		{
			name: "no arguments",
			fn: func() string {
				return "success"
			},
			args:        []interface{}{},
			expected:    []interface{}{"success"},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var args []reflect.Value
			for _, arg := range tt.args {
				args = append(args, reflect.ValueOf(arg))
			}

			results, err := Call(reflect.ValueOf(tt.fn), args)

			if tt.expectedErr != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedErr.Error(), err.Error())
				return
			}

			require.NoError(t, err)

			var actual []interface{}
			for _, result := range results {
				actual = append(actual, result.Interface())
			}

			require.Equal(t, len(tt.expected), len(actual), "number of results should match")
			for i := range actual {
				require.Equal(t, tt.expected[i], actual[i], "result at index %d should match", i)
			}
		})
	}
}
