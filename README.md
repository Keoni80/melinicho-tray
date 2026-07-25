# MeLi Nicho Tray

App de bandeja del sistema (Linux/macOS/Windows) que muestra las ventas de
hoy de [MeLi Nicho](https://melinicho.up.railway.app) y las actualiza sola
cada tanto (intervalo configurable).

- **macOS**: el monto aparece como texto en la barra de menús (nativo).
- **Windows**: el monto se dibuja como un ícono chico en la bandeja — no hay
  forma nativa de mostrar texto ancho ahí, así que se lee apretado.
- **Linux**: mismo problema que Windows con el ícono de bandeja (los íconos
  de app-indicator quedan chicos sin importar la resolución con la que se
  generen — es una limitación del plugin de bandeja del panel, no de la
  app). Para verlo grande y legible, la app también escribe el monto a
  `~/.config/melinicho-tray/genmon.txt` en el formato que espera el plugin
  **Generic Monitor** de XFCE — ver "Verlo grande en Linux (XFCE)" abajo.

Al pasar el mouse por el ícono (o abrir el menú) se ve también la
facturación del mes y la hora de la última actualización.

## Verlo grande en Linux (XFCE)

El ícono de la bandeja va a quedar chico sin importar qué se haga (ver
arriba). La alternativa es agregar un widget **Generic Monitor** al panel,
que sí muestra texto libre a cualquier tamaño:

1. Instalá el plugin si no lo tenés: `sudo apt install xfce4-genmon-plugin`
2. Clic derecho en el panel → Panel → Agregar nuevos elementos → **Generic
   Monitor** → Agregar
3. Clic derecho en el widget nuevo → Propiedades:
   - **Comando**: `cat /home/TU_USUARIO/.config/melinicho-tray/genmon.txt`
     (ajustá la ruta a tu usuario/SO)
   - **Período**: 15 segundos
   - Desmarcá "Usar etiqueta" si aparece tildado

El comando ya devuelve el texto con markup de Pango (`<txt>`) y el tooltip
(`<tool>`) — no hace falta tocar nada más.

## Instalación

Descargá el binario de tu plataforma desde
[Releases](https://github.com/Keoni80/melinicho-tray/releases) y ejecutalo.
No hace falta instalar nada más.

**macOS**: como el binario no está firmado por Apple, la primera vez
Gatekeeper probablemente lo bloquee. Click derecho → Abrir (en vez de doble
click), o corré `xattr -cr melinicho-tray-darwin-*` en una terminal antes de
abrirlo.

**Linux**: dale permiso de ejecución (`chmod +x melinicho-tray-linux-amd64`)
y corrélo. Necesita un panel con soporte de bandeja/appindicator (XFCE, la
mayoría de los entornos de escritorio lo tienen).

## Configuración

La primera vez que corre, crea un archivo de configuración vacío en:

- Linux: `~/.config/melinicho-tray/config.json`
- macOS: `~/Library/Application Support/melinicho-tray/config.json`
- Windows: `%AppData%\melinicho-tray\config.json`

Desde el menú de la bandeja (⚙️ Abrir carpeta de configuración) podés
llegar directo a ese archivo. Editalo con estos campos:

```json
{
  "server_url": "https://melinicho.up.railway.app",
  "username": "admin",
  "password": "tu-contraseña",
  "interval_seconds": 300
}
```

Después de guardarlo, elegí **🔁 Recargar configuración** en el menú de la
bandeja (no hace falta reiniciar la app).

`interval_seconds` es cada cuánto consulta a MeLi Nicho — 300 = cada 5
minutos. Un intervalo muy corto (por ejemplo, cada pocos segundos) puede
hacer que la API de MercadoLibre empiece a devolver errores de rate limit;
no hay problema con intervalos de 1 minuto o más.

## Compilar desde el código

Requiere Go 1.22+. En Linux además hace falta `libgtk-3-dev` y
`libayatana-appindicator3-dev` (o `libappindicator3-dev` en distros más
viejas).

```bash
go build .
```

Los binarios para las 3 plataformas se generan automáticamente vía GitHub
Actions (`.github/workflows/build.yml`) cuando se pushea un tag `v*` — no
hace falta tener una Mac o PC con Windows para generarlos.
