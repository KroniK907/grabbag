// Package main starts the Grab Bag host process.
package main

import "github.com/KroniK907/grabbag/internal/host"

// Windows PE resource object with requestedExecutionLevel asInvoker.
// Without it, unsigned Go binaries can trip installer detection and UAC.
//go:generate go run github.com/akavel/rsrc@v0.10.2 -arch amd64 -manifest windows.manifest -o rsrc_windows_amd64.syso

func main() {
	host.Main()
}
