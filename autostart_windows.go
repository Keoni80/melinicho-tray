//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const autostartSupported = true

// startupBatPath resuelve la ruta del .bat en la carpeta Startup de Windows
// (%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup) — todo lo que
// esté ahí arranca solo en cada login, sin tocar el Registro.
func startupBatPath() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", fmt.Errorf("no se encontró %%APPDATA%%")
	}
	return filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "Startup", "melinicho-tray.bat"), nil
}

func isAutostartEnabled() bool {
	path, err := startupBatPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

func enableAutostart() error {
	path, err := startupBatPath()
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return err
	}
	content := fmt.Sprintf("@echo off\r\nstart \"\" \"%s\"\r\n", exe)
	return os.WriteFile(path, []byte(content), 0644)
}

func disableAutostart() error {
	path, err := startupBatPath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	return err
}
