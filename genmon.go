package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// genmonPath es el archivo que el plugin "Generic Monitor" de XFCE lee
// (comando configurado en el plugin: `cat <esta ruta>`). Solo se usa en
// Linux, pero escribirlo en otras plataformas es inofensivo.
func genmonPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "genmon.txt"), nil
}

func genmonEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

// writeGenmonStatus arma la salida en el formato que espera genmon: un tag
// <txt> con markup de Pango (letra grande/negrita, más legible que el ícono
// chico de la bandeja) y un <tool> con el detalle completo para el tooltip.
func writeGenmonStatus(text, tooltip string) {
	path, err := genmonPath()
	if err != nil {
		return
	}
	content := fmt.Sprintf(
		"<txt><span size=\"large\" weight=\"bold\">%s</span></txt>\n<tool>%s</tool>\n",
		genmonEscape(text), genmonEscape(tooltip),
	)
	_ = os.WriteFile(path, []byte(content), 0644)
}
