package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

type SalesSummary struct {
	Today struct {
		Amount float64 `json:"amount"`
		Orders int     `json:"orders"`
	} `json:"today"`
	Month struct {
		Amount float64 `json:"amount"`
		Orders int     `json:"orders"`
	} `json:"month"`
	AsOf string `json:"as_of"`
}

type Client struct {
	cfg  *Config
	http *http.Client
}

func newClient(cfg *Config) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Jar:     jar,
			Timeout: 20 * time.Second,
		},
	}
}

func (c *Client) baseURL() string {
	return strings.TrimRight(c.cfg.ServerURL, "/")
}

// login hace POST a /login con las credenciales guardadas. La respuesta HTTP
// es 200 tanto si el login fue correcto (re-renderiza el login con éxito vía
// redirect a "/") como si falló (re-renderiza login.html con error) — Flask
// no distingue con el status code acá, así que el único chequeo confiable es
// pegarle después a un endpoint /api/* protegido y ver si sigue dando 401.
func (c *Client) login() error {
	form := url.Values{
		"username": {c.cfg.Username},
		"password": {c.cfg.Password},
	}
	resp, err := c.http.PostForm(c.baseURL()+"/login", form)
	if err != nil {
		return fmt.Errorf("no se pudo conectar a %s: %w", c.baseURL(), err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return nil
}

// fetchSalesSummary pega a /api/sales-summary; si la sesión expiró (401),
// reloguea una vez y reintenta.
func (c *Client) fetchSalesSummary() (*SalesSummary, error) {
	summary, status, err := c.doFetch()
	if err != nil {
		return nil, err
	}

	if status == http.StatusUnauthorized {
		if c.cfg.Username == "" || c.cfg.Password == "" {
			return nil, errors.New("faltan usuario/contraseña en la configuración")
		}
		if err := c.login(); err != nil {
			return nil, err
		}
		summary, status, err = c.doFetch()
		if err != nil {
			return nil, err
		}
		if status == http.StatusUnauthorized {
			return nil, errors.New("usuario o contraseña incorrectos")
		}
	}

	if status != http.StatusOK {
		return nil, fmt.Errorf("error del servidor (%d)", status)
	}
	return summary, nil
}

func (c *Client) doFetch() (*SalesSummary, int, error) {
	resp, err := c.http.Get(c.baseURL() + "/api/sales-summary")
	if err != nil {
		return nil, 0, fmt.Errorf("no se pudo conectar a %s: %w", c.baseURL(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return nil, resp.StatusCode, nil
	}

	var s SalesSummary
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("respuesta inesperada del servidor: %w", err)
	}
	return &s, resp.StatusCode, nil
}
