package domain

import (
	"fmt"
	"math"
	"strings"
)

var (
	unidades = []string{"", "UN", "DOS", "TRES", "CUATRO", "CINCO", "SEIS", "SIETE", "OCHO", "NUEVE"}
	decenas  = []string{"", "DIEZ", "VEINTE", "TREINTA", "CUARENTA", "CINCOENTA", "SESENTA", "SETENTA", "OCHENTA", "NOVENTA"}
	especial = []string{"DIEZ", "ONCE", "DOCE", "TRECE", "CATORCE", "QUINCE", "DIECISEIS", "DIECISIETE", "DIECIOCHO", "DIECINUEVE"}
	centenas = []string{"", "CIENTO", "DOSCIENTOS", "TRESCIENTOS", "CUATROCIENTOS", "QUINIENTOS", "SEISCIENTOS", "SETECIENTOS", "OCHO CIENTOS", "NOVECIENTOS"}
)

// NumeroALetras convierte una cantidad numérica a texto en español con formato de centavos (ej. 00/100)
func NumeroALetras(monto float64, moneda string) string {
	if monto == 0 {
		return fmt.Sprintf("CERO %s CON 00/100", getMonedaTexto(moneda))
	}

	entero := int64(math.Floor(monto))
	centavos := int(math.Round((monto - float64(entero)) * 100))

	textoEntero := convertirEntero(entero)
	if entero == 100 {
		textoEntero = "CIEN"
	}

	return fmt.Sprintf("%s %s CON %02d/100", textoEntero, getMonedaTexto(moneda), centavos)
}

func getMonedaTexto(moneda string) string {
	if strings.ToUpper(moneda) == "USD" {
		return "DÓLARES"
	}
	return "CÓRDOBAS"
}

func convertirEntero(n int64) string {
	if n == 0 {
		return ""
	}
	if n < 10 {
		return unidades[n]
	}
	if n >= 10 && n < 20 {
		return especial[n-10]
	}
	if n >= 20 && n < 30 {
		if n == 20 {
			return "VEINTE"
		}
		return "VEINTI" + unidades[n-20]
	}
	if n < 100 {
		d := n / 10
		u := n % 10
		if u == 0 {
			return decenas[d]
		}
		return decenas[d] + " Y " + unidades[u]
	}
	if n < 1000 {
		c := n / 100
		r := n % 100
		if c == 1 && r == 0 {
			return "CIEN"
		}
		return centenas[c] + " " + convertirEntero(r)
	}
	if n < 1000000 {
		m := n / 1000
		r := n % 1000
		prefix := "UN MIL"
		if m > 1 {
			prefix = convertirEntero(m) + " MIL"
		}
		if r == 0 {
			return prefix
		}
		return prefix + " " + convertirEntero(r)
	}

	return fmt.Sprintf("%d", n)
}
