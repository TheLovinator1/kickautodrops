// Package log provides a pluggable logging adapter.
//
// By default, output goes to stdout via fmt.Printf.
// Call SetLogger to redirect all output (e.g. to a TUI channel).
package log

import "fmt"

// LogFunc is the signature of the pluggable log function.
type LogFunc func(format string, args ...any)

var current LogFunc = func(format string, args ...any) {
	fmt.Printf(format, args...)
}

// SetLogger replaces the global log function. Pass nil to reset to default.
func SetLogger(f LogFunc) {
	if f == nil {
		current = func(format string, args ...any) { fmt.Printf(format, args...) }
	} else {
		current = f
	}
}

// Printf formats and writes a log message. A newline should be in the format.
func Printf(format string, args ...any) {
	current(format, args...)
}

// Println writes its arguments followed by a newline.
func Println(args ...any) {
	current(fmt.Sprintln(args...))
}
