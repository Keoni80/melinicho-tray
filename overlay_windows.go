//go:build windows

package main

import (
	"image"
	"image/color"
	"image/draw"
	"runtime"
	"sync"
	"unsafe"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/sys/windows"
)

const overlaySupported = true

var (
	ovG32                 = windows.NewLazySystemDLL("Gdi32.dll")
	ovPCreateDIBSection    = ovG32.NewProc("CreateDIBSection")
	ovPCreateCompatibleDC  = ovG32.NewProc("CreateCompatibleDC")
	ovPDeleteDC            = ovG32.NewProc("DeleteDC")
	ovPDeleteObject        = ovG32.NewProc("DeleteObject")
	ovPSelectObject        = ovG32.NewProc("SelectObject")

	ovK32              = windows.NewLazySystemDLL("Kernel32.dll")
	ovPGetModuleHandle = ovK32.NewProc("GetModuleHandleW")

	ovU32                  = windows.NewLazySystemDLL("User32.dll")
	ovPRegisterClass       = ovU32.NewProc("RegisterClassExW")
	ovPCreateWindowEx      = ovU32.NewProc("CreateWindowExW")
	ovPDefWindowProc       = ovU32.NewProc("DefWindowProcW")
	ovPDestroyWindow       = ovU32.NewProc("DestroyWindow")
	ovPShowWindow          = ovU32.NewProc("ShowWindow")
	ovPGetMessage          = ovU32.NewProc("GetMessageW")
	ovPTranslateMessage    = ovU32.NewProc("TranslateMessage")
	ovPDispatchMessage     = ovU32.NewProc("DispatchMessageW")
	ovPPostMessage         = ovU32.NewProc("PostMessageW")
	ovPGetDC               = ovU32.NewProc("GetDC")
	ovPReleaseDC           = ovU32.NewProc("ReleaseDC")
	ovPUpdateLayeredWindow = ovU32.NewProc("UpdateLayeredWindow")
	ovPGetSystemMetrics    = ovU32.NewProc("GetSystemMetrics")
	ovPGetWindowRect       = ovU32.NewProc("GetWindowRect")
	ovPSetWindowPos        = ovU32.NewProc("SetWindowPos")
)

const (
	ovWSPopup       = 0x80000000
	ovWSExLayered   = 0x00080000
	ovWSExTopmost   = 0x00000008
	ovWSExToolWin   = 0x00000080
	ovCWUseDefault  = -2147483648 // int32(0x80000000)
	ovSWShowNoAct   = 4
	ovSWHide        = 0
	ovWMDestroy     = 0x0002
	ovWMNCHitTest   = 0x0084
	ovHTCaption     = 2
	ovWMExitSize    = 0x0232
	ovWMApp         = 0x8000
	ovWMUpdateText  = ovWMApp + 1
	ovSMCXScreen    = 0
	ovSMCYScreen    = 1
	ovULWAlpha      = 0x00000002
	ovACSrcOver     = 0x00
	ovACSrcAlpha    = 0x01
	ovSWPNoActivate = 0x0010
	ovSWPNoZOrder   = 0x0004
)

type ovWndClassEx struct {
	Size, Style                        uint32
	WndProc                            uintptr
	ClsExtra, WndExtra                 int32
	Instance, Icon, Cursor, Background windows.Handle
	MenuName, ClassName                *uint16
	IconSm                             windows.Handle
}

type ovPoint struct{ X, Y int32 }
type ovSize struct{ CX, CY int32 }
type ovRect struct{ Left, Top, Right, Bottom int32 }

type ovBlendFunction struct {
	BlendOp, BlendFlags, SourceConstantAlpha, AlphaFormat byte
}

type ovBitmapInfoHeader struct {
	Size                                 uint32
	Width, Height                        int32
	Planes, BitCount                     uint16
	Compression, SizeImage               uint32
	XPelsPerMeter, YPelsPerMeter         int32
	ClrUsed, ClrImportant                uint32
}

var (
	overlayMu    sync.Mutex
	overlayHwnd  windows.Handle
	overlayBig   string
	overlaySmall string
	overlayW     int
	overlayH     int
)

const overlayClassName = "MelinichoOverlayWindow"

func startOverlay() {
	go overlayThread()
}

func overlayThread() {
	runtime.LockOSThread()

	hInstance, _, _ := ovPGetModuleHandle.Call(0)

	classNamePtr, _ := windows.UTF16PtrFromString(overlayClassName)
	wndProcCb := windows.NewCallback(overlayWndProc)

	wc := ovWndClassEx{
		WndProc:    wndProcCb,
		Instance:   windows.Handle(hInstance),
		ClassName:  classNamePtr,
		Background: 0,
	}
	wc.Size = uint32(unsafe.Sizeof(wc))
	ovPRegisterClass.Call(uintptr(unsafe.Pointer(&wc)))

	x, y, w, h := overlayInitialGeometry()

	titlePtr, _ := windows.UTF16PtrFromString("MeLi Nicho Overlay")
	hwnd, _, _ := ovPCreateWindowEx.Call(
		uintptr(ovWSExLayered|ovWSExTopmost|ovWSExToolWin),
		uintptr(unsafe.Pointer(classNamePtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		uintptr(ovWSPopup),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		0, 0, hInstance, 0,
	)
	if hwnd == 0 {
		return
	}

	overlayMu.Lock()
	overlayHwnd = windows.Handle(hwnd)
	overlayW, overlayH = w, h
	overlayMu.Unlock()

	redrawOverlay()

	mu.Lock()
	visible := cfg != nil && cfg.showOverlay()
	mu.Unlock()
	if visible {
		ovPShowWindow.Call(hwnd, ovSWShowNoAct)
	}

	var msg struct {
		Hwnd    windows.Handle
		Message uint32
		WParam  uintptr
		LParam  uintptr
		Time    uint32
		Pt      ovPoint
	}
	for {
		ret, _, _ := ovPGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		ovPTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		ovPDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

// overlayInitialGeometry calcula la posición/tamaño inicial: la guardada en
// config si es válida, o la esquina inferior derecha de la pantalla la
// primera vez (0,0 en el config nuevo significa "todavía no se guardó
// ninguna posición real" — nadie va a arrastrar la ventana justo a 0,0).
func overlayInitialGeometry() (x, y, w, h int) {
	w, h = 220, 90
	screenW, _, _ := ovPGetSystemMetrics.Call(ovSMCXScreen)
	screenH, _, _ := ovPGetSystemMetrics.Call(ovSMCYScreen)

	mu.Lock()
	savedX, savedY := 0, 0
	if cfg != nil {
		savedX, savedY = cfg.OverlayX, cfg.OverlayY
	}
	mu.Unlock()

	if savedX == 0 && savedY == 0 {
		x = int(screenW) - w - 24
		y = int(screenH) - h - 90 // encima de la bandeja/reloj de la taskbar
	} else {
		x, y = savedX, savedY
		if x < 0 || x > int(screenW)-40 {
			x = int(screenW) - w - 24
		}
		if y < 0 || y > int(screenH)-40 {
			y = int(screenH) - h - 90
		}
	}
	return x, y, w, h
}

func overlayWndProc(hwnd windows.Handle, msg uint32, wparam, lparam uintptr) uintptr {
	switch msg {
	case ovWMNCHitTest:
		return ovHTCaption
	case ovWMExitSize:
		persistOverlayPosition(hwnd)
		return 0
	case ovWMUpdateText:
		redrawOverlay()
		return 0
	case ovWMDestroy:
		return 0
	}
	ret, _, _ := ovPDefWindowProc.Call(uintptr(hwnd), uintptr(msg), wparam, lparam)
	return ret
}

func persistOverlayPosition(hwnd windows.Handle) {
	var r ovRect
	ovPGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&r)))
	mu.Lock()
	if cfg != nil {
		cfg.OverlayX = int(r.Left)
		cfg.OverlayY = int(r.Top)
		_ = saveConfig(cfg)
	}
	mu.Unlock()
}

// updateOverlayText actualiza el texto mostrado; es seguro llamarla desde
// cualquier goroutine (guarda el texto bajo lock y solo despierta el loop
// de mensajes de la ventana con un PostMessage — el redibujado real corre
// siempre en el thread que la creó, como exige Win32).
func updateOverlayText(big, small string) {
	overlayMu.Lock()
	overlayBig = big
	overlaySmall = small
	hwnd := overlayHwnd
	overlayMu.Unlock()
	if hwnd != 0 {
		ovPPostMessage.Call(uintptr(hwnd), ovWMUpdateText, 0, 0)
	}
}

func showOverlayWindow() {
	overlayMu.Lock()
	hwnd := overlayHwnd
	overlayMu.Unlock()
	if hwnd != 0 {
		ovPShowWindow.Call(uintptr(hwnd), ovSWShowNoAct)
	}
}

func hideOverlayWindow() {
	overlayMu.Lock()
	hwnd := overlayHwnd
	overlayMu.Unlock()
	if hwnd != 0 {
		ovPShowWindow.Call(uintptr(hwnd), ovSWHide)
	}
}

func redrawOverlay() {
	overlayMu.Lock()
	hwnd := overlayHwnd
	big := overlayBig
	small := overlaySmall
	overlayMu.Unlock()
	if hwnd == 0 {
		big, small = "MeLi Nicho", "cargando…"
	}

	img := renderOverlayPanel(big, small)
	w := img.Bounds().Dx()
	h := img.Bounds().Dy()

	screenDC, _, _ := ovPGetDC.Call(0)
	defer ovPReleaseDC.Call(0, screenDC)

	memDC, _, _ := ovPCreateCompatibleDC.Call(screenDC)
	defer ovPDeleteDC.Call(memDC)

	bi := ovBitmapInfoHeader{
		Width:       int32(w),
		Height:      -int32(h), // negativo: top-down DIB
		Planes:      1,
		BitCount:    32,
		Compression: 0, // BI_RGB
	}
	bi.Size = uint32(unsafe.Sizeof(bi))

	var bits unsafe.Pointer
	hBitmap, _, _ := ovPCreateDIBSection.Call(
		memDC,
		uintptr(unsafe.Pointer(&bi)),
		0, // DIB_RGB_COLORS
		uintptr(unsafe.Pointer(&bits)),
		0, 0,
	)
	if hBitmap == 0 {
		return
	}
	defer ovPDeleteObject.Call(hBitmap)

	oldObj, _, _ := ovPSelectObject.Call(memDC, hBitmap)
	defer ovPSelectObject.Call(memDC, oldObj)

	// Windows espera BGRA con alpha premultiplicado para UpdateLayeredWindow.
	dst := unsafe.Slice((*byte)(bits), w*h*4)
	for py := 0; py < h; py++ {
		for px := 0; px < w; px++ {
			r, g, b, a := img.At(px, py).RGBA()
			i := (py*w + px) * 4
			dst[i+0] = byte(b >> 8)
			dst[i+1] = byte(g >> 8)
			dst[i+2] = byte(r >> 8)
			dst[i+3] = byte(a >> 8)
		}
	}

	x, y := overlayOrigin(hwnd)
	srcPt := ovPoint{0, 0}
	dstPt := ovPoint{int32(x), int32(y)}
	size := ovSize{int32(w), int32(h)}
	blend := ovBlendFunction{BlendOp: ovACSrcOver, SourceConstantAlpha: 255, AlphaFormat: ovACSrcAlpha}

	ovPUpdateLayeredWindow.Call(
		uintptr(hwnd),
		0,
		uintptr(unsafe.Pointer(&dstPt)),
		uintptr(unsafe.Pointer(&size)),
		memDC,
		uintptr(unsafe.Pointer(&srcPt)),
		0,
		uintptr(unsafe.Pointer(&blend)),
		ovULWAlpha,
	)

	if hwnd != 0 {
		ovPSetWindowPos.Call(uintptr(hwnd), 0, uintptr(x), uintptr(y), uintptr(w), uintptr(h), ovSWPNoActivate|ovSWPNoZOrder)
		overlayMu.Lock()
		overlayW, overlayH = w, h
		overlayMu.Unlock()
	}
}

func overlayOrigin(hwnd windows.Handle) (int, int) {
	if hwnd == 0 {
		return 0, 0
	}
	var r ovRect
	ovPGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&r)))
	return int(r.Left), int(r.Top)
}

// renderOverlayPanel dibuja el panel flotante: fondo oscuro semitransparente
// con esquinas redondeadas, monto de hoy grande arriba y el mes chico abajo
// — mismo criterio de contorno que renderIcon() en icon.go, reusa la misma
// fuente ya parseada (parsedFont, init() en icon.go).
func renderOverlayPanel(big, small string) *image.RGBA {
	const padX = 16
	const padY = 12
	const bigSize = 26
	const smallSize = 14
	const radius = 12
	const minWidth = 160

	bigFace := truetype.NewFace(parsedFont, &truetype.Options{Size: bigSize, DPI: 72})
	smallFace := truetype.NewFace(parsedFont, &truetype.Options{Size: smallSize, DPI: 72})
	defer bigFace.Close()
	defer smallFace.Close()

	bigW := measureText(bigFace, big)
	smallW := measureText(smallFace, small)
	width := bigW
	if smallW > width {
		width = smallW
	}
	width += padX * 2
	if width < minWidth {
		width = minWidth
	}
	height := padY*2 + bigSize + 6 + smallSize + 4

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), image.Transparent, image.Point{}, draw.Src)
	drawRoundedRect(img, img.Bounds(), radius, color.RGBA{20, 20, 24, 210})

	ctx := freetype.NewContext()
	ctx.SetDPI(72)
	ctx.SetFont(parsedFont)
	ctx.SetClip(img.Bounds())
	ctx.SetDst(img)

	ctx.SetFontSize(bigSize)
	ctx.SetSrc(image.NewUniform(color.White))
	ctx.DrawString(big, freetype.Pt(padX, padY+bigSize))

	ctx.SetFontSize(smallSize)
	ctx.SetSrc(image.NewUniform(color.RGBA{200, 200, 205, 255}))
	ctx.DrawString(small, freetype.Pt(padX, padY+bigSize+6+smallSize))

	return img
}

func measureText(face font.Face, s string) int {
	var w int
	for _, r := range s {
		aw, ok := face.GlyphAdvance(r)
		if !ok {
			continue
		}
		w += aw.Round()
	}
	return w
}

func drawRoundedRect(img *image.RGBA, rect image.Rectangle, radius int, col color.RGBA) {
	minX, minY, maxX, maxY := rect.Min.X, rect.Min.Y, rect.Max.X, rect.Max.Y
	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			if inRoundedRect(x, y, minX, minY, maxX, maxY, radius) {
				img.SetRGBA(x, y, col)
			}
		}
	}
}

func inRoundedRect(x, y, minX, minY, maxX, maxY, r int) bool {
	if x >= minX+r && x < maxX-r {
		return y >= minY && y < maxY
	}
	if y >= minY+r && y < maxY-r {
		return x >= minX && x < maxX
	}
	corners := [4][2]int{
		{minX + r, minY + r},
		{maxX - r - 1, minY + r},
		{minX + r, maxY - r - 1},
		{maxX - r - 1, maxY - r - 1},
	}
	for _, c := range corners {
		dx, dy := x-c[0], y-c[1]
		if dx*dx+dy*dy <= r*r {
			return true
		}
	}
	return false
}
