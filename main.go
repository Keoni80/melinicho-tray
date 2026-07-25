package main

import (
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/getlantern/systray"
)

var (
	mu         sync.Mutex
	cfg        *Config
	client     *Client
	menuStatus *systray.MenuItem
)

func main() {
	systray.Run(onReady, onExit)
}

var linuxDotIcon []byte // ícono fijo chico para Linux; se arma en onReady (no como var package-level: correría antes del init() que parsea la fuente en icon.go)

// setStatusIcon actualiza lo que se ve en la bandeja/barra de menús.
func setStatusIcon(text string) {
	switch runtime.GOOS {
	case "darwin":
		// SetTitle es texto real de la barra de menús en Mac (nítido, no
		// atado al tamaño de un ícono bitmap chico); el ícono queda de
		// respaldo/complemento.
		systray.SetTemplateIcon(renderTemplateIcon(text), renderIcon(text))
		systray.SetTitle(text)
	case "linux":
		// El ícono de bandeja en Linux (app-indicator) queda chico sin
		// importar la resolución con la que se genere — es una limitación
		// del plugin de bandeja del panel, no de esta app. El monto grande
		// y legible se muestra aparte, vía writeGenmonStatus(), en un
		// widget "Generic Monitor" del panel (texto real, no ícono). Acá
		// alcanza con un ícono fijo chico como punto de entrada al menú.
		systray.SetIcon(linuxDotIcon)
	default: // windows: no hay SetTitle ni genmon, todo depende del ícono bitmap
		systray.SetIcon(renderIcon(text))
	}
}

func onReady() {
	linuxDotIcon = renderIcon("$")
	setStatusIcon("...")
	systray.SetTooltip("MeLi Nicho — cargando...")

	loaded, needsSetup, err := loadConfig()
	if err != nil {
		log.Printf("error cargando config: %v", err)
	}
	mu.Lock()
	cfg = loaded
	client = newClient(cfg)
	mu.Unlock()

	menuStatus = systray.AddMenuItem("Cargando…", "")
	menuStatus.Disable()
	systray.AddSeparator()
	mRefresh := systray.AddMenuItem("🔄 Actualizar ahora", "Consultar MeLi Nicho ahora")
	mFolder := systray.AddMenuItem("⚙️ Abrir carpeta de configuración", "Editar server_url / usuario / contraseña / intervalo")
	mReload := systray.AddMenuItem("🔁 Recargar configuración", "Releer config.json después de editarlo")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("❌ Salir", "Cerrar MeLi Nicho Tray")

	if needsSetup {
		showConfigNeeded()
	} else {
		go refresh()
	}

	go func() {
		for {
			mu.Lock()
			interval := cfg.IntervalSeconds
			mu.Unlock()
			time.Sleep(time.Duration(interval) * time.Second)
			refresh()
		}
	}()

	go func() {
		for {
			select {
			case <-mRefresh.ClickedCh:
				go refresh()
			case <-mFolder.ClickedCh:
				openConfigFolder()
			case <-mReload.ClickedCh:
				reloadConfig()
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

func onExit() {}

func refresh() {
	mu.Lock()
	c := client
	mu.Unlock()

	summary, err := c.fetchSalesSummary()
	if err != nil {
		showError(err)
		return
	}

	text := formatCompact(summary.Today.Amount)
	setStatusIcon(text)

	tooltip := fmt.Sprintf(
		"Hoy: $%.0f (%d órdenes)\nMes: $%.0f (%d órdenes)\nActualizado %s",
		summary.Today.Amount, summary.Today.Orders,
		summary.Month.Amount, summary.Month.Orders,
		summary.AsOf,
	)
	systray.SetTooltip(tooltip)
	writeGenmonStatus(text, tooltip)
	if menuStatus != nil {
		menuStatus.SetTitle(fmt.Sprintf("Hoy $%.0f · Mes $%.0f (act. %s)", summary.Today.Amount, summary.Month.Amount, summary.AsOf))
	}
}

func showError(err error) {
	log.Printf("error consultando melinicho: %v", err)
	setStatusIcon("!")
	systray.SetTooltip("Error consultando MeLi Nicho: " + err.Error())
	writeGenmonStatus("Error", "Error consultando MeLi Nicho: "+err.Error())
	if menuStatus != nil {
		menuStatus.SetTitle("⚠ Error: " + err.Error())
	}
}

func showConfigNeeded() {
	path, _ := configPath()
	setStatusIcon("CFG")
	systray.SetTooltip("Falta configurar usuario/contraseña — abrí la carpeta de configuración desde el menú")
	writeGenmonStatus("Config", "Falta configurar usuario/contraseña — abrí la carpeta de configuración desde el menú")
	if menuStatus != nil {
		menuStatus.SetTitle("⚠ Configurá usuario/contraseña")
	}
	log.Printf("Config incompleto, completá %s y elegí 'Recargar configuración'", path)
}

func openConfigFolder() {
	dir, err := configDir()
	if err != nil {
		return
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", dir)
	case "windows":
		cmd = exec.Command("explorer", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	_ = cmd.Start()
}

func reloadConfig() {
	loaded, needsSetup, err := loadConfig()
	if err != nil {
		showError(err)
		return
	}
	mu.Lock()
	cfg = loaded
	client = newClient(cfg)
	mu.Unlock()

	if needsSetup {
		showConfigNeeded()
		return
	}
	go refresh()
}
