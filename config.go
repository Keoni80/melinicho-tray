package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	ServerURL       string `json:"server_url"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	IntervalSeconds int    `json:"interval_seconds"`
	// ShowOverlay es puntero para distinguir "no estaba en el config.json"
	// (nil -> default true) de "el usuario lo puso en false" — un bool
	// simple no podría distinguir esos dos casos al leer un config.json
	// viejo de antes de que existiera este campo.
	ShowOverlay *bool `json:"show_overlay,omitempty"`
	OverlayX    int   `json:"overlay_x"`
	OverlayY    int   `json:"overlay_y"`
}

func (c *Config) showOverlay() bool {
	return c.ShowOverlay == nil || *c.ShowOverlay
}

const defaultServerURL = "https://melinicho.up.railway.app"
const defaultInterval = 300 // 5 minutos

func configDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "melinicho-tray"), nil
}

func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// loadConfig lee el config.json. Si no existe, crea uno con valores por
// defecto (usuario/contraseña vacíos) y lo devuelve junto con needsSetup=true
// para que el llamador sepa que hay que avisarle al usuario que lo complete.
func loadConfig() (*Config, bool, error) {
	path, err := configPath()
	if err != nil {
		return nil, false, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		cfg := &Config{
			ServerURL:       defaultServerURL,
			Username:        "",
			Password:        "",
			IntervalSeconds: defaultInterval,
		}
		if err := saveConfig(cfg); err != nil {
			return nil, false, err
		}
		return cfg, true, nil
	}
	if err != nil {
		return nil, false, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, false, fmt.Errorf("config.json inválido: %w", err)
	}
	if cfg.ServerURL == "" {
		cfg.ServerURL = defaultServerURL
	}
	if cfg.IntervalSeconds <= 0 {
		cfg.IntervalSeconds = defaultInterval
	}
	needsSetup := cfg.Username == "" || cfg.Password == ""
	return &cfg, needsSetup, nil
}

func saveConfig(cfg *Config) error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	path, err := configPath()
	if err != nil {
		return err
	}
	// 0600: el archivo tiene la contraseña de admin en texto plano.
	return os.WriteFile(path, data, 0600)
}
