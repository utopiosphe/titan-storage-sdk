package utils

import (
	"context"
	"fmt"
	"reflect"
	"time"
)

const (
	defaultMaxAttempts    = 5
	defaultInitialBackoff = 50 * time.Millisecond
	backoffTimes          = 5
)

// BackoffCall wraps any function whose last return value is error,
// with configurable retries and exponential backoff.
type BackoffCall struct {
	fn             reflect.Value
	maxAttempts    int
	initialBackoff time.Duration
}

// Option configures a BackoffCall.
type Option func(*BackoffCall)

// WithMaxAttempts sets how many times to invoke the function (including the first).
func WithMaxAttempts(n int) Option {
	return func(b *BackoffCall) {
		b.maxAttempts = n
	}
}

// WithInitialBackoff sets the initial backoff duration; each retry doubles it.
func WithInitialBackoff(d time.Duration) Option {
	return func(b *BackoffCall) {
		b.initialBackoff = d
	}
}

// NewBackoffCall creates a BackoffCall for fn, applying any Options.
// Panics if fn is not a function.
func NewBackoffCall(fn interface{}, opts ...Option) *BackoffCall {
	vfn := reflect.ValueOf(fn)
	if vfn.Kind() != reflect.Func {
		panic(fmt.Sprintf("NewBackoffCall: fn must be a function, got %T", fn))
	}
	b := &BackoffCall{
		fn: vfn,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// BackInterfaces holds the return values of a function call (including its error as the last element).
type BackInterfaces []interface{}

// Start invokes the function with provided ctx and args. It returns a slice of return-values on success,
// or an error after exhausting all attempts.
func (b *BackoffCall) Start(ctx context.Context, args ...interface{}) (BackInterfaces, error) {

	if b.maxAttempts <= 0 {
		b.maxAttempts = defaultMaxAttempts
	}

	if b.initialBackoff <= 0 {
		b.initialBackoff = defaultInitialBackoff
	}

	backoff := b.initialBackoff
	var lastErr error

	fnType := b.fn.Type()
	numIn := fnType.NumIn()
	in := make([]reflect.Value, numIn)
	for i := 0; i < numIn; i++ {
		if i < len(args) && args[i] != nil {
			in[i] = reflect.ValueOf(args[i])
		} else {
			in[i] = reflect.Zero(fnType.In(i)) // zero value handle avoid nil pointer panic
		}
	}

	for attempt := 1; attempt <= b.maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		out := b.fn.Call(in)
		errVal := out[len(out)-1] // assume last return is error

		if errVal.IsNil() {
			res := make([]interface{}, len(out))
			for i, o := range out {
				res[i] = o.Interface()
			}
			return res, nil
		}

		lastErr = errVal.Interface().(error)
		if attempt < b.maxAttempts {
			time.Sleep(backoff)
			backoff *= 2
		}
		fmt.Printf("backoff %d: %v\n", attempt, lastErr)
	}

	return nil, fmt.Errorf("after %d attempts, last error: %w", b.maxAttempts, lastErr)
}

// ToDest assigns the elements of BackInterfaces (excluding the last error element)
// to the provided dest pointers and returns the final error element.
// The number of dest pointers must equal len(bi)-1.
func (bi BackInterfaces) ToDest(dest ...interface{}) error {
	expected := len(bi) - 1
	if len(dest) != expected {
		return fmt.Errorf("ToDest: expected %d dest arguments, got %d", expected, len(dest))
	}
	for i, d := range dest {
		ptrVal := reflect.ValueOf(d)
		if ptrVal.Kind() != reflect.Ptr {
			return fmt.Errorf("ToDest: dest[%d] must be a pointer, got %T", i, d)
		}
		val := reflect.ValueOf(bi[i])
		if !val.Type().AssignableTo(ptrVal.Elem().Type()) {
			return fmt.Errorf("ToDest: cannot assign value of type %s to dest[%d] of type %s", val.Type(), i, ptrVal.Elem().Type())
		}
		ptrVal.Elem().Set(val)
	}
	// extract and return the final error element
	last := bi[len(bi)-1]
	if last == nil {
		return nil
	}
	err, ok := last.(error)
	if !ok {
		return fmt.Errorf("ToDest: last element is not error: %T", last)
	}
	return err
}
