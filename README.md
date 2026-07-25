# MeLi Nicho Tray

App de bandeja del sistema (Linux/macOS/Windows) que muestra las ventas de
hoy de [MeLi Nicho](https://melinicho.up.railway.app) y las actualiza sola
cada tanto (intervalo configurable).

- **macOS**: el monto aparece como texto en la barra de menús (nativo).
- **Linux/Windows**: el monto se dibuja como un ícono chico en la bandeja —
  no hay forma nativa de mostrar texto ancho en esas plataformas, así que es
  más apretado que en Mac.

Al pasar el mouse por el ícono (o abrir el menú) se ve también la
facturación del mes y la hora de la última actualización.

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
