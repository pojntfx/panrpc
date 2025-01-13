package rpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBasicClosureCreationAndCall(t *testing.T) {
	m := &closureManager{
		closures: make(map[string]func(args ...interface{}) (interface{}, error)),
	}

	fn := func(ctx context.Context, x int) (int, error) {
		return x * 2, nil
	}

	closureID, cleanup, err := registerClosure(m, fn)
	require.NoError(t, err)
	defer cleanup()

	result, err := m.CallClosure(context.Background(), closureID, []interface{}{42})
	require.NoError(t, err)
	require.Equal(t, 84, result)
}

func TestClosureWithJustErrorReturn(t *testing.T) {
	m := &closureManager{
		closures: make(map[string]func(args ...interface{}) (interface{}, error)),
	}

	expectedErr := assert.AnError
	fn := func(ctx context.Context) error {
		return expectedErr
	}

	closureID, cleanup, err := registerClosure(m, fn)
	require.NoError(t, err)
	defer cleanup()

	result, err := m.CallClosure(context.Background(), closureID, []interface{}{})
	require.ErrorIs(t, err, expectedErr)
	require.Nil(t, result)
}

func TestInvalidFunctionSignatureNoReturnValues(t *testing.T) {
	m := &closureManager{
		closures: make(map[string]func(args ...interface{}) (interface{}, error)),
	}

	// Function with no return values
	fn := func() {}

	_, _, err := registerClosure(m, fn)
	require.ErrorIs(t, err, ErrInvalidReturn)
}

func TestInvalidFunctionSignatureOneInvalidReturn(t *testing.T) {
	m := &closureManager{
		closures: make(map[string]func(args ...interface{}) (interface{}, error)),
	}

	// Function with only a non-error return value
	fn := func() bool {
		return false
	}

	_, _, err := registerClosure(m, fn)
	require.ErrorIs(t, err, ErrInvalidReturn)
}

func TestInvalidFunctionSignatureTwoInvalidReturns(t *testing.T) {
	m := &closureManager{
		closures: make(map[string]func(args ...interface{}) (interface{}, error)),
	}

	// Function with two invalid return values since the second return value isn't an error
	fn := func() (bool, bool) {
		return false, false
	}

	_, _, err := registerClosure(m, fn)
	require.ErrorIs(t, err, ErrInvalidReturn)
}

func TestInvalidFunctionSignatureMoreThanTwoReturns(t *testing.T) {
	m := &closureManager{
		closures: make(map[string]func(args ...interface{}) (interface{}, error)),
	}

	// Function with three return values
	fn := func() (bool, bool, bool) {
		return false, false, false
	}

	_, _, err := registerClosure(m, fn)
	require.ErrorIs(t, err, ErrInvalidReturn)
}

func TestCleanupRemovesClosure(t *testing.T) {
	m := &closureManager{
		closures: make(map[string]func(args ...interface{}) (interface{}, error)),
	}

	fn := func(ctx context.Context) error {
		return nil
	}

	closureID, cleanup, err := registerClosure(m, fn)
	require.NoError(t, err)

	// Call cleanup to remove the closure
	cleanup()

	// Try to call the removed closure
	_, err = m.CallClosure(context.Background(), closureID, []interface{}{})
	require.ErrorIs(t, err, ErrClosureDoesNotExist)
}

func TestInvalidArgumentCount(t *testing.T) {
	m := &closureManager{
		closures: make(map[string]func(args ...interface{}) (interface{}, error)),
	}

	fn := func(ctx context.Context, x int) error {
		return nil
	}

	closureID, cleanup, err := registerClosure(m, fn)
	require.NoError(t, err)
	defer cleanup()

	// Call with wrong number of arguments
	_, err = m.CallClosure(context.Background(), closureID, []interface{}{42, 43})
	require.ErrorIs(t, err, ErrInvalidArgsCount)
}

func TestInvalidArgumentType(t *testing.T) {
	m := &closureManager{
		closures: make(map[string]func(args ...interface{}) (interface{}, error)),
	}

	fn := func(ctx context.Context, x int) error {
		return nil
	}

	closureID, cleanup, err := registerClosure(m, fn)
	require.NoError(t, err)
	defer cleanup()

	// Call with wrong argument type (string instead of int)
	_, err = m.CallClosure(context.Background(), closureID, []interface{}{"not an int"})
	require.ErrorIs(t, err, ErrInvalidArg)
}

func TestNonExistentClosure(t *testing.T) {
	m := &closureManager{
		closures: make(map[string]func(args ...interface{}) (interface{}, error)),
	}

	_, err := m.CallClosure(context.Background(), "non-existent-id", []interface{}{})
	require.ErrorIs(t, err, ErrClosureDoesNotExist)
}

func TestConcurrentClosureCalls(t *testing.T) {
	m := &closureManager{
		closures: make(map[string]func(args ...interface{}) (interface{}, error)),
	}

	fn := func(ctx context.Context, x int) (int, error) {
		return x * 2, nil
	}

	closureID, cleanup, err := registerClosure(m, fn)
	require.NoError(t, err)
	defer cleanup()

	// Run multiple goroutines calling the same closure
	const numGoroutines = 10
	done := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		go func(val int) {
			result, err := m.CallClosure(context.Background(), closureID, []interface{}{val})
			require.NoError(t, err)
			require.Equal(t, val*2, result)
			done <- struct{}{}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

func TestContextCancellation(t *testing.T) {
	m := &closureManager{
		closures: make(map[string]func(args ...interface{}) (interface{}, error)),
	}

	fn := func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}

	closureID, cleanup, err := registerClosure(m, fn)
	require.NoError(t, err)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())

	// Start the closure in a goroutine
	done := make(chan error)
	go func() {
		_, err := m.CallClosure(ctx, closureID, []interface{}{})
		done <- err
	}()

	// Cancel the context
	cancel()

	// Check if the closure returned with context cancellation error
	err = <-done
	require.ErrorIs(t, err, context.Canceled)
}
