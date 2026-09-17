//go:build windows
// +build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	winspool          = syscall.NewLazyDLL("winspool.drv")
	procOpenPrinter   = winspool.NewProc("OpenPrinterW")
	procStartDocDoc   = winspool.NewProc("StartDocPrinterW")
	procStartPage     = winspool.NewProc("StartPagePrinter")
	procWritePrinter  = winspool.NewProc("WritePrinter")
	procEndPage       = winspool.NewProc("EndPagePrinter")
	procEndDoc       = winspool.NewProc("EndDocPrinter")
	procClosePrinter  = winspool.NewProc("ClosePrinter")
)

type DOC_INFO_1 struct {
	pDocName    *uint16
	pOutputFile *uint16
	pDatatype   *uint16
}

// PrintRawToWindowsPrinter envía bytes binarios crudos (ESC/P2) directamente a la impresora usando winspool.drv
func PrintRawToPrinter(printerName string, rawBytes []byte) error {
	var handle syscall.Handle

	printerNamePtr, err := syscall.UTF16PtrFromString(printerName)
	if err != nil {
		return fmt.Errorf("error al convertir nombre de impresora: %w", err)
	}

	// 1. OpenPrinter
	ret, _, err := procOpenPrinter.Call(
		uintptr(unsafe.Pointer(printerNamePtr)),
		uintptr(unsafe.Pointer(&handle)),
		0,
	)
	if ret == 0 {
		return fmt.Errorf("no se pudo abrir la impresora %s: %v", printerName, err)
	}
	defer procClosePrinter.Call(uintptr(handle))

	// 2. StartDocPrinter (Tipo de datos RAW para comandos ESC/P2 directo)
	docNamePtr, _ := syscall.UTF16PtrFromString("SIFACO_Factura_ESC_P2")
	dataTypePtr, _ := syscall.UTF16PtrFromString("RAW")

	docInfo := DOC_INFO_1{
		pDocName:    docNamePtr,
		pOutputFile: nil,
		pDatatype:   dataTypePtr,
	}

	ret, _, err = procStartDocDoc.Call(
		uintptr(handle),
		1,
		uintptr(unsafe.Pointer(&docInfo)),
	)
	if ret == 0 {
		return fmt.Errorf("error en StartDocPrinter: %v", err)
	}
	defer procEndDoc.Call(uintptr(handle))

	// 3. StartPagePrinter
	ret, _, err = procStartPage.Call(uintptr(handle))
	if ret == 0 {
		return fmt.Errorf("error en StartPagePrinter: %v", err)
	}
	defer procEndPage.Call(uintptr(handle))

	// 4. WritePrinter (Escritura de los bytes ESC/P2 crudos al puerto USB)
	var written uint32
	ret, _, err = procWritePrinter.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&rawBytes[0])),
		uintptr(len(rawBytes)),
		uintptr(unsafe.Pointer(&written)),
	)

	if ret == 0 || written != uint32(len(rawBytes)) {
		return fmt.Errorf("error al escribir bytes RAW en la impresora: impresos %d de %d bytes, err: %v", written, len(rawBytes), err)
	}

	return nil
}
