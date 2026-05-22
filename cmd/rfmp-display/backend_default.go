//go:build !hw && !sim

package main

import (
	"fmt"
	"os"
)

func newDisplay() Display {
	fmt.Fprintln(os.Stderr, "ERROR: no backend selected. Build with -tags sim or -tags hw")
	os.Exit(1)
	return nil
}

func newKeyboard() Keyboard {
	fmt.Fprintln(os.Stderr, "ERROR: no backend selected. Build with -tags sim or -tags hw")
	os.Exit(1)
	return nil
}

func newLED() LED {
	fmt.Fprintln(os.Stderr, "ERROR: no backend selected. Build with -tags sim or -tags hw")
	os.Exit(1)
	return nil
}
