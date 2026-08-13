//go:build !windows

package main

// El diálogo nativo para cambiar el intervalo es Win32-only (ver
// settings_windows.go) — en Linux/macOS se sigue editando interval_seconds
// a mano en config.json (ver README), no-op acá para que main.go no
// necesite build tags propios.
const intervalConfigSupported = false

func promptIntervalSeconds(current int) (int, bool) {
	return current, false
}
