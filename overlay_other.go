//go:build !windows

package main

// El overlay flotante es Win32-only (ver overlay_windows.go) — en
// Linux/macOS ya existen sus propios mecanismos para mostrar el monto
// grande y legible (genmon en Linux, SetTitle nativo en macOS), así que acá
// alcanza con no-ops para que main.go no necesite build tags propios.
const overlaySupported = false

func startOverlay()                       {}
func updateOverlayText(big, small string) {}
func showOverlayWindow()                  {}
func hideOverlayWindow()                  {}
