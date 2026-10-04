// Package shelltest drives the /board and / shells in headless Chromium.
// The tests start a real host, join phones through the UI, and check what a
// player would see: no reloads, the right tenant mounted, nothing left
// running after unmount, and nothing room-private on a locked board. They
// skip when no Chromium is installed.
package shelltest
