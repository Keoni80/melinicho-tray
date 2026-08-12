//go:build !windows

package main

import "errors"

// Linux ya tiene su propio mecanismo de autostart (melinicho-tray.desktop,
// ver README) y macOS no lo pide todavía — estos stubs solo existen para que
// main.go compile en todas las plataformas sin `#ifdef`s dispersos.
const autostartSupported = false

func isAutostartEnabled() bool { return false }
func enableAutostart() error   { return errors.New("no soportado en este sistema") }
func disableAutostart() error  { return errors.New("no soportado en este sistema") }
