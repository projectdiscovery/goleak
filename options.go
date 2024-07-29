// Copyright (c) 2017-2023 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package goleak

import (
	"strings"
	"time"

	"github.com/projectdiscovery/goleak/stack"
)

// Option lets users specify custom verifications.
type Option interface {
	apply(*opts)
}

// We retry up to 20 times if we can't find the goroutine that
// we are looking for. In between each attempt, we will sleep for
// a short while to let any running goroutines complete.
const _defaultRetries = 20

type opts struct {
	filters    []func(stack.Stack) bool
	maxRetries int
	maxSleep   time.Duration
	cleanup    func(int)
	pretty     bool
}

// implement apply so that opts struct itself can be used as
// an Option.
func (o *opts) apply(opts *opts) {
	opts.filters = o.filters
	opts.maxRetries = o.maxRetries
	opts.maxSleep = o.maxSleep
	opts.cleanup = o.cleanup
}

// optionFunc lets us easily write options without a custom type.
type optionFunc func(*opts)

func (f optionFunc) apply(opts *opts) { f(opts) }

// IgnoreTopFunction ignores any goroutines where the specified function
// is at the top of the stack. The function name should be fully qualified,
// e.g., github.com/projectdiscovery/goleak.IgnoreTopFunction
func IgnoreTopFunction(f string) Option {
	return addFilter(func(s stack.Stack) bool {
		return s.FirstFunction() == f
	})
}

// Pretty sets the output of the leak check to be more human-readable.
func Pretty() Option {
	return optionFunc(func(opts *opts) {
		opts.pretty = true
	})
}

// IgnoreAnyFunction ignores goroutines where the specified function
// is present anywhere in the stack.
//
// The function name must be fully qualified, e.g.,
//
//	github.com/projectdiscovery/goleak.IgnoreAnyFunction
//
// For methods, the fully qualified form looks like:
//
//	github.com/projectdiscovery/goleak.(*MyType).MyMethod
func IgnoreAnyFunction(f string) Option {
	return addFilter(func(s stack.Stack) bool {
		return s.HasFunction(f)
	})
}

func IgnoreAnyEntry(e string) Option {
	return addFilter(func(s stack.Stack) bool {
		return s.MatchAnyEntry(e)
	})
}

// ignoreAnyFunctionMatching ignores goroutines where any function
// matches the specified regular expression.
//
// The accuracy depends on the regular expression. That is why given regex
// should be as specific as possible.
// One should use IgnoreAnyContainingPackage or IgnoreAnyContainingStruct
// instead of this function if possible.
func ignoreAnyFunctionMatching(regex string) Option {
	return addFilter(func(s stack.Stack) bool {
		return s.MatchAnyFunction(regex)
	})
}

// IgnoreAnyContainingPkg creates an option that filters out goroutines
// if any function in their stack trace includes the specified package name.
// The package name must be fully qualified, such as "github.com/projectdiscovery/goleak".
// Note: The package name does not require escaping in this context.
func IgnoreAnyContainingPkg(pkg string) Option {
	return ignoreAnyFunctionMatching(`\Q` + pkg + `.\E.+`)
}

// IgnoreAnyContainingStruct provides an option to filter out goroutines based on the presence of a specified struct
// in any function within their stack trace. The struct name must be fully qualified, such as "github.com/projectdiscovery/goleak.(*MyType)".
// Note: The struct name should be used as is without any need for escaping special characters.
func IgnoreAnyContainingStruct(str string) Option {
	return ignoreAnyFunctionMatching(`\Q` + str + `.\E.+`)
}

// IncludeAllContainingPkg filters goroutines to only include those where any function
// contains the specified package name. This is useful for focusing on goroutines
// originating from specific packages, particularly in unit tests.
//
// The package name must be fully qualified, e.g., "github.com/projectdiscovery/goleak".
//
// Example use case:
// This function can be used to focus on goroutines that are relevant to the user's
// own packages, excluding those from third-party packages.
func IncludeAllContainingPkg(pkg string) Option {
	return addFilter(func(s stack.Stack) bool {
		// Construct a regex pattern that matches the fully qualified package name
		// and checks if any function in the stack trace includes this package.
		pattern := `\Q` + pkg + `.\E.+`
		return s.MatchAnyFunction(pattern)
	})
}

// Cleanup sets up a cleanup function that will be executed at the
// end of the leak check.
// When passed to [VerifyTestMain], the exit code passed to cleanupFunc
// will be set to the exit code of TestMain.
// When passed to [VerifyNone], the exit code will be set to 0.
// This cannot be passed to [Find].
func Cleanup(cleanupFunc func(exitCode int)) Option {
	return optionFunc(func(opts *opts) {
		opts.cleanup = cleanupFunc
	})
}

// IgnoreCurrent records all current goroutines when the option is created, and ignores
// them in any future Find/Verify calls.
func IgnoreCurrent() Option {
	excludeIDSet := map[int]bool{}
	for _, s := range stack.All() {
		excludeIDSet[s.ID()] = true
	}
	return addFilter(func(s stack.Stack) bool {
		return excludeIDSet[s.ID()]
	})
}

func maxSleep(d time.Duration) Option {
	return optionFunc(func(opts *opts) {
		opts.maxSleep = d
	})
}

func addFilter(f func(stack.Stack) bool) Option {
	return optionFunc(func(opts *opts) {
		opts.filters = append(opts.filters, f)
	})
}

func buildOpts(options ...Option) *opts {
	opts := &opts{
		maxRetries: _defaultRetries,
		maxSleep:   100 * time.Millisecond,
	}
	opts.filters = append(opts.filters,
		isTestStack,
		isSyscallStack,
		isStdLibStack,
		isTraceStack,
	)
	for _, option := range options {
		option.apply(opts)
	}
	return opts
}

func buildOnlyOpts(options ...Option) *opts {
	opts := &opts{
		maxRetries: _defaultRetries,
		maxSleep:   100 * time.Millisecond,
	}
	for _, option := range options {
		option.apply(opts)
	}
	return opts
}

func (o *opts) filter(s stack.Stack) bool {
	for _, filter := range o.filters {
		if filter(s) {
			return true
		}
	}
	return false
}

func (o *opts) retry(i int) bool {
	if i >= o.maxRetries {
		return false
	}

	d := time.Duration(int(time.Microsecond) << uint(i))
	if d > o.maxSleep {
		d = o.maxSleep
	}
	time.Sleep(d)
	return true
}

// isTestStack is a default filter installed to automatically skip goroutines
// that the testing package runs while the user's tests are running.
func isTestStack(s stack.Stack) bool {
	// Until go1.7, the main goroutine ran RunTests, which started
	// the test in a separate goroutine and waited for that test goroutine
	// to end by waiting on a channel.
	// Since go1.7, a separate goroutine is started to wait for signals.
	// T.Parallel is for parallel tests, which are blocked until all serial
	// tests have run with T.Parallel at the top of the stack.
	// testing.runFuzzTests is for fuzz testing, it's blocked until the test
	// function with all seed corpus have run.
	// testing.runFuzzing is for fuzz testing, it's blocked until a failing
	// input is found.
	switch s.FirstFunction() {
	case "testing.RunTests", "testing.(*T).Run", "testing.(*T).Parallel", "testing.runFuzzing", "testing.runFuzzTests":
		// In pre1.7 and post-1.7, background goroutines started by the testing
		// package are blocked waiting on a channel.
		return strings.HasPrefix(s.State(), "chan receive")
	}
	return false
}

func isSyscallStack(s stack.Stack) bool {
	// Typically runs in the background when code uses CGo:
	// https://github.com/golang/go/issues/16714
	return s.HasFunction("runtime.goexit") && strings.HasPrefix(s.State(), "syscall")
}

func isStdLibStack(s stack.Stack) bool {
	// Importing os/signal starts a background goroutine.
	// The name of the function at the top has changed between versions.
	if f := s.FirstFunction(); f == "os/signal.signal_recv" || f == "os/signal.loop" {
		return true
	}

	// Using signal.Notify will start a runtime goroutine.
	return s.HasFunction("runtime.ensureSigM")
}
