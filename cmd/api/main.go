package main

import (
	"context"
	"log"
	"os"

	handlerHttp "sifaco/backend/internal/handler/http"
	"sifaco/backend/internal/repository/postgres"
	"sifaco/backend/internal/service"
)

func main() {
	log.Println("Iniciando Servidor Backend SIFACO (Nicaragua DGI)...")

	// Cadena de conexión predeterminada para entorno de desarrollo local
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/sifaco?sslmode=disable"
	}

	ctx := context.Background()
	repo, err := postgres.NewRepository(ctx, dbURL)
	if err != nil {
		log.Printf("ADVERTENCIA: No se pudo conectar a PostgreSQL (%v). El servidor iniciará pero requerirá BD activa.", err)
	} else {
		defer repo.Close()
		log.Println("Conexión exitosa a PostgreSQL 16.")
	}

	// Inicializar Servicios
	factSvc := service.NewFacturacionService(repo)
	cartSvc := service.NewCarteraService(repo)
	repSvc := service.NewReportesDGIService(repo)
	cliSvc := service.NewClienteService(repo)
	audSvc := service.NewAuditoriaService(repo)
	drpSvc := service.NewDRPService(repo, audSvc)
	usrSvc := service.NewUsuarioService(repo)

	// Inicializar Router HTTP (Gin)
	r := handlerHttp.NewRouter(factSvc, cartSvc, repSvc, cliSvc, audSvc, drpSvc, usrSvc)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Servidor API SIFACO escuchando en http://localhost:%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Error al iniciar el servidor: %v", err)
	}
}
