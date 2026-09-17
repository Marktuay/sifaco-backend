//go:build !windows
// +build !windows

package main

import (
	"log"
)

// PrintRawToPrinter es una implementación simulada para entornos fuera de Windows (macOS/Linux dev)
func PrintRawToPrinter(printerName string, rawBytes []byte) error {
	log.Printf("[SIMULACIÓN IMPRESIÓN RAW] Recibidos %d bytes binarios ESC/P2 para la impresora '%s'", len(rawBytes), printerName)
	log.Printf("[SIMULACIÓN RAW CONTENIDO]:\n%s", string(rawBytes))
	return nil
}
