package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

type PrintRequest struct {
	PayloadBase64 string `json:"payload_base64"`
	Impresora     string `json:"impresora"` // Opcional, por defecto "Epson LQ-590"
}

func main() {
	log.Println("Iniciando Agente de Impresión Local SIFACO (Epson LQ-590 ESC/P2)...")

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"estado":    "ONLINE",
			"impresora": "Epson LQ-590 (24-pin ESC/P2)",
			"puerto":    "9100",
		})
	})

	http.HandleFunc("/imprimir", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, "Método no permitido", http.StatusMethodNotAllowed)
			return
		}

		var req PrintRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido: "+err.Error(), http.StatusBadRequest)
			return
		}

		if req.PayloadBase64 == "" {
			http.Error(w, "Debe proporcionar 'payload_base64'", http.StatusBadRequest)
			return
		}

		rawBytes, err := base64.StdEncoding.DecodeString(req.PayloadBase64)
		if err != nil {
			http.Error(w, "Error al decodificar base64: "+err.Error(), http.StatusBadRequest)
			return
		}

		printerName := req.Impresora
		if printerName == "" {
			printerName = "Epson LQ-590"
		}

		log.Printf("Recibida orden de impresión de %d bytes para impresora '%s'", len(rawBytes), printerName)

		err = PrintRawToPrinter(printerName, rawBytes)
		if err != nil {
			log.Printf("ERROR de Impresión: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": fmt.Sprintf("Falló el envío de comandos RAW a la impresora: %v", err),
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"exito":          true,
			"mensaje":        "Formulario enviado exitosamente a la impresora Epson LQ-590",
			"bytes_enviados": len(rawBytes),
		})
	})

	port := os.Getenv("AGENT_PORT")
	if port == "" {
		port = "9100"
	}

	log.Printf("Agente de impresión escuchando en http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Error fatal en agente de impresión: %v", err)
	}
}

func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}
