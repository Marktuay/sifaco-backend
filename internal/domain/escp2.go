package domain

import (
	"bytes"
	"fmt"
	"strings"
	"time"
)

// ESC/P2 Command Constants for Epson LQ-590 (24-pin Impact Matrix Printer)
const (
	ESC = "\x1B"
	FF  = "\x0C" // FormFeed (Avanza la página al siguiente formulario preimpreso)
	CR  = "\x0D"
	LF  = "\x0A"
)

// GenerarPayloadESCP2LQ590 produce una secuencia de bytes binarios crudos ESC/P2
// perfectamente alineada a los campos del formulario preimpreso de Facturación RINSA (Factura N° 4135).
func GenerarPayloadESCP2LQ590(factura Factura, cliente Cliente, detalles []FacturaDetalle, cuotas []FacturaCuota) []byte {
	var buf bytes.Buffer

	// 1. Reset e Inicialización de Impresora Epson LQ-590
	buf.WriteString(ESC + "@")     // Reset de fábrica
	buf.WriteString(ESC + "x\x01") // Select Letter Quality (LQ 24-pin)
	buf.WriteString(ESC + "k\x01") // Select Font Sans Serif
	buf.WriteString(ESC + "P")     // Select 10 CPI (Characters Per Inch)
	buf.WriteString(ESC + "C\x42") // Configurar largo de página a 66 líneas (11 pulgadas a 6 LPI)

	currentLine := 1

	// Función auxiliar para avanzar la impresora hasta el renglón deseado
	gotoLine := func(targetLine int) {
		for currentLine < targetLine {
			buf.WriteString(LF)
			currentLine++
		}
	}

	// Función auxiliar para posicionar en una columna (10 CPI) agregando espacios
	formatCol := func(col int, text string) string {
		if col <= 1 {
			return text
		}
		return strings.Repeat(" ", col-1) + text
	}

	// Line 6: Correlativo de Factura (Esquina superior derecha, Col 68)
	gotoLine(6)
	buf.WriteString(formatCol(68, factura.CorrelativoPreimpreso) + LF)
	currentLine++

	// Line 8: Fecha de Emisión y Moneda (Col 15 y Col 40)
	gotoLine(8)
	fechaStr := factura.FechaEmision.Format("02/01/2006")
	line8Str := formatCol(15, fmt.Sprintf("%-12s", fechaStr)) + formatCol(13, fmt.Sprintf("MONEDA: %-3s  TASA BCN: %.4f", factura.Moneda, factura.TasaCambioBCN))
	buf.WriteString(line8Str + LF)
	currentLine++

	// Line 10: Razón Social del Cliente (Col 15) y RUC/Cédula (Col 60)
	gotoLine(10)
	clienteNombre := truncateString(cliente.RazonSocial, 42)
	line10Str := formatCol(15, fmt.Sprintf("%-42s", clienteNombre)) + formatCol(3, fmt.Sprintf("%-20s", cliente.RuccEdula))
	buf.WriteString(line10Str + LF)
	currentLine++

	// Line 12: Dirección (Col 15) y Teléfono (Col 60)
	gotoLine(12)
	direccion := truncateString(cliente.Direccion, 42)
	line12Str := formatCol(15, fmt.Sprintf("%-42s", direccion)) + formatCol(3, fmt.Sprintf("%-20s", cliente.Telefono))
	buf.WriteString(line12Str + LF)
	currentLine++

	// Line 14: Condición de Pago (Col 18)
	gotoLine(14)
	buf.WriteString(formatCol(18, factura.TipoPago) + LF)
	currentLine++

	// Tabla de Items (Renglones 21 a 45)
	itemStartLine := 21
	gotoLine(itemStartLine)

	maxItems := 10
	printedItems := 0

	for _, d := range detalles {
		if printedItems >= maxItems {
			break
		}
		exentoTag := " "
		if d.Exento {
			exentoTag = "(E)"
		}
		desc := truncateString(d.Descripcion, 35)

		// Formato de columnas: Cant (Col 5) | Desc (Col 16) | P.Unit (Col 54) | Total (Col 68)
		rowLine := formatCol(5, fmt.Sprintf("%4.0f", d.Cantidad)) +
			formatCol(7, fmt.Sprintf("%-35s %s", desc, exentoTag)) +
			formatCol(3, fmt.Sprintf("%12.2f", d.PrecioUnitario)) +
			formatCol(2, fmt.Sprintf("%13.2f", d.Subtotal))

		buf.WriteString(rowLine + LF)
		currentLine++
		printedItems++
	}

	// Line 48: Monto en Letras (Col 15)
	gotoLine(48)
	buf.WriteString(formatCol(15, "SON: "+truncateString(factura.MontoLetras, 65)) + LF)
	currentLine++

	// Totales en el bloque lateral derecho (Lines 51, 53, 55, 57 en Col 68)
	subtotalTotal := factura.SubtotalGravado + factura.SubtotalExento
	montoDescuento := 0.0 // Descuento global si lo hubiere
	gotoLine(51)
	buf.WriteString(formatCol(68, fmt.Sprintf("%13.2f", subtotalTotal)) + LF)
	currentLine++

	gotoLine(53)
	buf.WriteString(formatCol(68, fmt.Sprintf("%13.2f", montoDescuento)) + LF)
	currentLine++

	gotoLine(55)
	buf.WriteString(formatCol(68, fmt.Sprintf("%13.2f", factura.MontoIVA)) + LF)
	currentLine++

	gotoLine(57)
	buf.WriteString(formatCol(68, fmt.Sprintf("%13.2f", factura.Total)) + LF)
	currentLine++

	// 7. Expulsión de hoja con FormFeed (Avanza exacto al inicio del siguiente formulario)
	buf.WriteString(FF)

	return buf.Bytes()
}

// GenerarPayloadESCP2ReciboLQ590 produce una secuencia de bytes binarios crudos ESC/P2
// alineada al formulario preimpreso de Recibo Oficial de Caja RINSA (Recibo N° 5411).
func GenerarPayloadESCP2ReciboLQ590(req RegistrarReciboMultiFacturaRequest, cliente Cliente, montoLetras string, totalPagado float64, saldoPendiente float64) []byte {
	var buf bytes.Buffer

	// 1. Reset e Inicialización de Impresora Epson LQ-590
	buf.WriteString(ESC + "@")     // Reset de fábrica
	buf.WriteString(ESC + "x\x01") // Select Letter Quality (LQ 24-pin)
	buf.WriteString(ESC + "k\x01") // Select Font Sans Serif
	buf.WriteString(ESC + "P")     // Select 10 CPI (Characters Per Inch)
	buf.WriteString(ESC + "C\x42") // Configurar largo de página a 66 líneas (11 pulgadas a 6 LPI)

	currentLine := 1

	gotoLine := func(targetLine int) {
		for currentLine < targetLine {
			buf.WriteString(LF)
			currentLine++
		}
	}

	formatCol := func(col int, text string) string {
		if col <= 1 {
			return text
		}
		return strings.Repeat(" ", col-1) + text
	}

	// Line 4: Fecha (Col 10), Código Cliente (Col 38), Recibo N° (Col 66)
	gotoLine(4)
	fechaStr := req.Fecha.Format("02/01/2006")
	codigoCliente := req.CodigoCliente
	if codigoCliente == "" {
		codigoCliente = truncateString(cliente.RuccEdula, 15)
	}
	line4Str := formatCol(10, fmt.Sprintf("%-12s", fechaStr)) +
		formatCol(16, fmt.Sprintf("%-15s", codigoCliente)) +
		formatCol(13, fmt.Sprintf("%-12s", req.NumeroRecibo))
	buf.WriteString(line4Str + LF)
	currentLine++

	// Line 7: Recibimos De (Col 18)
	gotoLine(7)
	nombreCliente := req.RecibimosDe
	if nombreCliente == "" {
		nombreCliente = cliente.RazonSocial
	}
	buf.WriteString(formatCol(18, truncateString(nombreCliente, 60)) + LF)
	currentLine++

	// Line 9: La Suma De (Monto en Letras) (Col 18)
	gotoLine(9)
	buf.WriteString(formatCol(18, truncateString(montoLetras, 60)) + LF)
	currentLine++

	// Line 11: En Concepto De (Col 18)
	gotoLine(11)
	buf.WriteString(formatCol(18, truncateString(req.Concepto, 60)) + LF)
	currentLine++

	// Tabla Multi-factura (Lines 15 a 22 - máx 8 facturas)
	gotoLine(15)
	maxRows := 8
	printedRows := 0

	for _, item := range req.Facturas {
		if printedRows >= maxRows {
			break
		}
		fFecha := item.FechaFactura
		if len(fFecha) > 10 {
			fFecha = fFecha[:10]
		}
		// Cols: Fecha (Col 5) | FactN° (Col 16) | SubTotal (Col 27) | N°NC (Col 37) | MontoNC (Col 45) | %Desc (Col 54) | Desc (Col 60) | TotalPagado (Col 69)
		rowLine := formatCol(5, fmt.Sprintf("%-10s", fFecha)) +
			formatCol(1, fmt.Sprintf("%-10s", truncateString(item.NumeroFactura, 10))) +
			formatCol(1, fmt.Sprintf("%9.2f", item.Subtotal)) +
			formatCol(1, fmt.Sprintf("%-7s", truncateString(item.NumeroNotaCredito, 7))) +
			formatCol(1, fmt.Sprintf("%8.2f", item.MontoNotaCredito)) +
			formatCol(1, fmt.Sprintf("%5.1f", item.PorcentajeDesc)) +
			formatCol(1, fmt.Sprintf("%8.2f", item.MontoDescuento)) +
			formatCol(1, fmt.Sprintf("%12.2f", item.TotalPagado))

		buf.WriteString(rowLine + LF)
		currentLine++
		printedRows++
	}

	// Resumen y Forma de Pago (Lines 26, 28, 30)
	gotoLine(26)
	// Left: Banco (Col 12), Right: Saldo C$ (Col 68)
	bancoStr := truncateString(req.Banco, 15)
	line26Str := formatCol(12, fmt.Sprintf("%-15s", bancoStr)) + formatCol(41, fmt.Sprintf("%12.2f", saldoPendiente))
	buf.WriteString(line26Str + LF)
	currentLine++

	gotoLine(28)
	// Left: N° Ref (Col 12), Right: Efectivo C$ (Col 68)
	refStr := truncateString(req.NumeroRef, 15)
	line28Str := formatCol(12, fmt.Sprintf("%-15s", refStr)) + formatCol(41, fmt.Sprintf("%12.2f", totalPagado))
	buf.WriteString(line28Str + LF)
	currentLine++

	gotoLine(30)
	// Left: Valor (Col 12), Right: TOTAL C$ (Col 68)
	line30Str := formatCol(12, fmt.Sprintf("%15.2f", totalPagado)) + formatCol(41, fmt.Sprintf("%12.2f", totalPagado))
	buf.WriteString(line30Str + LF)
	currentLine++

	// 7. Expulsión de hoja con FormFeed
	buf.WriteString(FF)

	return buf.Bytes()
}

func truncateString(str string, num int) string {
	bn := []rune(str)
	if len(bn) > num {
		return string(bn[:num])
	}
	return str
}

// FormatDate Utility
func FormatDate(t time.Time) string {
	return t.Format("02/01/2006")
}
