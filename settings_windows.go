//go:build windows

package main

import (
	"runtime"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const intervalConfigSupported = true
const minIntervalSeconds = 15

var (
	stgPPostQuitMessage  = ovU32.NewProc("PostQuitMessage")
	stgPIsDialogMessage  = ovU32.NewProc("IsDialogMessageW")
	stgPGetWindowText    = ovU32.NewProc("GetWindowTextW")
	stgPSendMessage      = ovU32.NewProc("SendMessageW")
	stgPSetFocus         = ovU32.NewProc("SetFocus")
	stgPMessageBox       = ovU32.NewProc("MessageBoxW")
	stgPGetStockObject   = ovG32.NewProc("GetStockObject")
)

const (
	stgWMCommand        = 0x0111
	stgWMClose          = 0x0010
	stgWMSetFont        = 0x0030
	stgEMSetSel         = 0x00B1
	stgIDOK             = 1
	stgIDCancel         = 2
	stgBSDefPushButton  = 0x0001
	stgESNumber         = 0x2000
	stgESAutoHScroll    = 0x0080
	stgWSChild          = 0x40000000
	stgWSVisible        = 0x10000000
	stgWSBorder         = 0x00800000
	stgWSTabStop        = 0x00010000
	stgWSCaption        = 0x00C00000
	stgWSSysMenu        = 0x00080000
	stgDefaultGuiFont   = 17
	stgMBOkIconWarning  = 0x30
)

type intervalDialogState struct {
	hEdit  windows.Handle
	result int
	ok     bool
}

var (
	settingsClassOnce   sync.Once
	activeIntervalMu    sync.Mutex
	activeIntervalState *intervalDialogState
)

func registerSettingsClass(hInstance uintptr) *uint16 {
	classNamePtr, _ := windows.UTF16PtrFromString("MelinichoIntervalDialog")
	settingsClassOnce.Do(func() {
		cb := windows.NewCallback(intervalDialogWndProc)
		wc := ovWndClassEx{
			WndProc:    cb,
			Instance:   windows.Handle(hInstance),
			ClassName:  classNamePtr,
			Background: windows.Handle(6), // COLOR_WINDOW+1
		}
		wc.Size = uint32(unsafe.Sizeof(wc))
		ovPRegisterClass.Call(uintptr(unsafe.Pointer(&wc)))
	})
	return classNamePtr
}

func intervalDialogWndProc(hwnd windows.Handle, msg uint32, wparam, lparam uintptr) uintptr {
	activeIntervalMu.Lock()
	st := activeIntervalState
	activeIntervalMu.Unlock()

	switch msg {
	case stgWMCommand:
		id := uint16(wparam & 0xFFFF)
		switch id {
		case stgIDOK:
			if st == nil {
				break
			}
			text := getWindowText(st.hEdit)
			n, err := strconv.Atoi(strings.TrimSpace(text))
			if err != nil || n < minIntervalSeconds {
				showWarning(hwnd, "Ingresá un número entero de al menos "+strconv.Itoa(minIntervalSeconds)+" segundos.")
				return 0
			}
			st.result = n
			st.ok = true
			ovPDestroyWindow.Call(uintptr(hwnd))
			return 0
		case stgIDCancel:
			ovPDestroyWindow.Call(uintptr(hwnd))
			return 0
		}
	case stgWMClose:
		ovPDestroyWindow.Call(uintptr(hwnd))
		return 0
	case ovWMDestroy:
		stgPPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := ovPDefWindowProc.Call(uintptr(hwnd), uintptr(msg), wparam, lparam)
	return ret
}

func showWarning(owner windows.Handle, text string) {
	textPtr, _ := windows.UTF16PtrFromString(text)
	titlePtr, _ := windows.UTF16PtrFromString("Valor inválido")
	stgPMessageBox.Call(uintptr(owner), uintptr(unsafe.Pointer(textPtr)), uintptr(unsafe.Pointer(titlePtr)), stgMBOkIconWarning)
}

func getWindowText(hwnd windows.Handle) string {
	buf := make([]uint16, 32)
	stgPGetWindowText.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return windows.UTF16ToString(buf)
}

// promptIntervalSeconds abre un diálogo nativo simple (label + campo
// numérico + Guardar/Cancelar) para elegir cada cuánto se consulta
// MeLi Nicho. Corre su propio message loop (bloqueante, como un modal) en
// el thread que la llama — se espera que el caller sea la goroutine que
// procesa clicks del menú, no la que corre refresh() en loop.
func promptIntervalSeconds(current int) (int, bool) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hInstance, _, _ := ovPGetModuleHandle.Call(0)
	classNamePtr := registerSettingsClass(hInstance)

	const w, h = 340, 160
	screenW, _, _ := ovPGetSystemMetrics.Call(ovSMCXScreen)
	screenH, _, _ := ovPGetSystemMetrics.Call(ovSMCYScreen)
	x := (int(screenW) - w) / 2
	y := (int(screenH) - h) / 2

	titlePtr, _ := windows.UTF16PtrFromString("Frecuencia de actualización — MeLi Nicho Tray")
	hwndDlg, _, _ := ovPCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(classNamePtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		uintptr(stgWSCaption|stgWSSysMenu|stgWSVisible),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		0, 0, hInstance, 0,
	)
	if hwndDlg == 0 {
		return current, false
	}

	font, _, _ := stgPGetStockObject.Call(stgDefaultGuiFont)

	labelPtr, _ := windows.UTF16PtrFromString("Cada cuántos segundos consultar (mínimo " + strconv.Itoa(minIntervalSeconds) + "):")
	hLabel, _, _ := ovPCreateWindowEx.Call(0,
		uintptr(unsafe.Pointer(strPtr("STATIC"))), uintptr(unsafe.Pointer(labelPtr)),
		uintptr(stgWSChild|stgWSVisible),
		20, 16, 290, 20, hwndDlg, 0, hInstance, 0)
	stgPSendMessage.Call(hLabel, stgWMSetFont, font, 1)

	valuePtr, _ := windows.UTF16PtrFromString(strconv.Itoa(current))
	hEdit, _, _ := ovPCreateWindowEx.Call(uintptr(stgWSBorder),
		uintptr(unsafe.Pointer(strPtr("EDIT"))), uintptr(unsafe.Pointer(valuePtr)),
		uintptr(stgWSChild|stgWSVisible|stgWSTabStop|stgESNumber|stgESAutoHScroll),
		20, 44, 290, 24, hwndDlg, 0, hInstance, 0)
	stgPSendMessage.Call(hEdit, stgWMSetFont, font, 1)

	okPtr, _ := windows.UTF16PtrFromString("Guardar")
	hOK, _, _ := ovPCreateWindowEx.Call(0,
		uintptr(unsafe.Pointer(strPtr("BUTTON"))), uintptr(unsafe.Pointer(okPtr)),
		uintptr(stgWSChild|stgWSVisible|stgWSTabStop|stgBSDefPushButton),
		120, 90, 90, 28, hwndDlg, uintptr(stgIDOK), hInstance, 0)
	stgPSendMessage.Call(hOK, stgWMSetFont, font, 1)

	cancelPtr, _ := windows.UTF16PtrFromString("Cancelar")
	hCancel, _, _ := ovPCreateWindowEx.Call(0,
		uintptr(unsafe.Pointer(strPtr("BUTTON"))), uintptr(unsafe.Pointer(cancelPtr)),
		uintptr(stgWSChild|stgWSVisible|stgWSTabStop),
		220, 90, 90, 28, hwndDlg, uintptr(stgIDCancel), hInstance, 0)
	stgPSendMessage.Call(hCancel, stgWMSetFont, font, 1)

	st := &intervalDialogState{hEdit: windows.Handle(hEdit), result: current, ok: false}
	activeIntervalMu.Lock()
	activeIntervalState = st
	activeIntervalMu.Unlock()

	stgPSetFocus.Call(hEdit)
	stgPSendMessage.Call(hEdit, stgEMSetSel, 0, ^uintptr(0)) // seleccionar todo el texto

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
		handled, _, _ := stgPIsDialogMessage.Call(uintptr(hwndDlg), uintptr(unsafe.Pointer(&msg)))
		if handled == 0 {
			ovPTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			ovPDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}

	activeIntervalMu.Lock()
	activeIntervalState = nil
	activeIntervalMu.Unlock()

	return st.result, st.ok
}

func strPtr(s string) *uint16 {
	p, _ := windows.UTF16PtrFromString(s)
	return p
}
