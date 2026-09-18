package service

import (
	"context"
	"fmt"
	"time"

	"sifaco/backend/internal/domain"
	"sifaco/backend/internal/repository/postgres"

	"github.com/google/uuid"
)

type ClienteService struct {
	Repo *postgres.Repository
}

func NewClienteService(repo *postgres.Repository) *ClienteService {
	s := &ClienteService{Repo: repo}
	s.syncDefaultClientesToDB()
	return s
}

func (s *ClienteService) syncDefaultClientesToDB() {
	if s.Repo == nil || s.Repo.Pool == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	seedClientes := []domain.Cliente{
		{
			ID:                  uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a01"),
			RuccEdula:          "J0310000001234",
			RazonSocial:        "COMPAÑÍA DISTRIBUIDORA DE NICARAGUA S.A.",
			Direccion:          "Km 6.5 Carretera Norte, Managua",
			Telefono:           "2255-8800",
			Email:              "contacto@cdn.com.ni",
			RepresentanteLegal: "Lic. Carlos Mendoza",
			TipoContribuyente:  "GRAN_CONTRIBUYENTE",
		},
		{
			ID:                  uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a02"),
			RuccEdula:          "J0310000005678",
			RazonSocial:        "COMERCIALIZADORA DEL PACÍFICO S.A.",
			Direccion:          "De la Iglesia Zaragoza 2c Abajo, León",
			Telefono:           "2311-4455",
			Email:              "ventas@compacifico.com.ni",
			RepresentanteLegal: "Dra. Elena Rostrán",
			TipoContribuyente:  "REGIMEN_GENERAL",
		},
		{
			ID:                  uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a03"),
			RuccEdula:          "J0310000009999",
			RazonSocial:        "DESARROLLOS Y EVENTOS RINSA S.A.",
			Direccion:          "Reparto Bolonia, Managua",
			Telefono:           "2277-1122",
			Email:              "eventos@rinsa.com.ni",
			RepresentanteLegal: "Ing. Fernando Gutiérrez",
			TipoContribuyente:  "GRAN_CONTRIBUYENTE",
		},
	}

	for _, c := range seedClientes {
		query := `INSERT INTO clientes (id, ruc_cedula, razon_social, direccion, telefono, email, representante_legal, tipo_contribuyente)
				  VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
				  ON CONFLICT (ruc_cedula) DO NOTHING`
		_, _ = s.Repo.Pool.Exec(ctx, query, c.ID, c.RuccEdula, c.RazonSocial, c.Direccion, c.Telefono, c.Email, c.RepresentanteLegal, c.TipoContribuyente)
	}
}

func (s *ClienteService) ListarClientes(ctx context.Context, busqueda string) ([]domain.Cliente, error) {
	if s == nil || s.Repo == nil || s.Repo.Pool == nil {
		c1ID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a01")
		c2ID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a02")
		c3ID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a03")
		return []domain.Cliente{
			{
				ID:                  c1ID,
				RuccEdula:          "J0310000001234",
				RazonSocial:        "COMPAÑÍA DISTRIBUIDORA DE NICARAGUA S.A.",
				Direccion:          "Km 6.5 Carretera Norte, Managua",
				Telefono:           "2255-8800",
				Email:              "contacto@cdn.com.ni",
				RepresentanteLegal: "Lic. Carlos Mendoza",
				TipoContribuyente:  "GRAN_CONTRIBUYENTE",
			},
			{
				ID:                  c2ID,
				RuccEdula:          "J0310000005678",
				RazonSocial:        "COMERCIALIZADORA DEL PACÍFICO S.A.",
				Direccion:          "De la Iglesia Zaragoza 2c Abajo, León",
				Telefono:           "2311-4455",
				Email:              "ventas@compacifico.com.ni",
				RepresentanteLegal: "Dra. Elena Rostrán",
				TipoContribuyente:  "REGIMEN_GENERAL",
			},
			{
				ID:                  c3ID,
				RuccEdula:          "J0310000009999",
				RazonSocial:        "DESARROLLOS Y EVENTOS RINSA S.A.",
				Direccion:          "Reparto Bolonia, Managua",
				Telefono:           "2277-1122",
				Email:              "eventos@rinsa.com.ni",
				RepresentanteLegal: "Ing. Fernando Gutiérrez",
				TipoContribuyente:  "GRAN_CONTRIBUYENTE",
			},
		}, nil
	}

	query := `SELECT id, ruc_cedula, razon_social, COALESCE(direccion, ''), COALESCE(telefono, ''), COALESCE(email, ''), COALESCE(representante_legal, ''), tipo_contribuyente, creado_en
	          FROM clientes`

	var args []interface{}
	if busqueda != "" {
		query += ` WHERE ruc_cedula ILIKE $1 OR razon_social ILIKE $1 OR COALESCE(representante_legal, '') ILIKE $1`
		args = append(args, "%"+busqueda+"%")
	}
	query += ` ORDER BY razon_social ASC`

	rows, err := s.Repo.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("error al listar clientes: %w", err)
	}
	defer rows.Close()

	clientes := make([]domain.Cliente, 0)
	for rows.Next() {
		var c domain.Cliente
		if err := rows.Scan(&c.ID, &c.RuccEdula, &c.RazonSocial, &c.Direccion, &c.Telefono, &c.Email, &c.RepresentanteLegal, &c.TipoContribuyente, &c.CreadoEn); err != nil {
			return nil, fmt.Errorf("error al escanear cliente: %w", err)
		}
		clientes = append(clientes, c)
	}

	return clientes, nil
}

func (s *ClienteService) ObtenerClientePorID(ctx context.Context, id uuid.UUID) (*domain.Cliente, error) {
	if s == nil || s.Repo == nil || s.Repo.Pool == nil {
		return nil, fmt.Errorf("base de datos no disponible")
	}

	var c domain.Cliente
	err := s.Repo.Pool.QueryRow(ctx,
		`SELECT id, ruc_cedula, razon_social, COALESCE(direccion, ''), COALESCE(telefono, ''), COALESCE(email, ''), COALESCE(representante_legal, ''), tipo_contribuyente, creado_en
		 FROM clientes WHERE id = $1`, id).
		Scan(&c.ID, &c.RuccEdula, &c.RazonSocial, &c.Direccion, &c.Telefono, &c.Email, &c.RepresentanteLegal, &c.TipoContribuyente, &c.CreadoEn)

	if err != nil {
		return nil, fmt.Errorf("cliente no encontrado: %w", err)
	}
	return &c, nil
}

func (s *ClienteService) CrearCliente(ctx context.Context, req domain.CrearClienteRequest) (*domain.Cliente, error) {
	if s == nil || s.Repo == nil || s.Repo.Pool == nil {
		return nil, fmt.Errorf("base de datos no disponible")
	}

	if req.TipoContribuyente == "" {
		req.TipoContribuyente = "GRAN_CONTRIBUYENTE"
	}

	var c domain.Cliente
	err := s.Repo.Pool.QueryRow(ctx,
		`INSERT INTO clientes (ruc_cedula, razon_social, direccion, telefono, email, representante_legal, tipo_contribuyente)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, ruc_cedula, razon_social, COALESCE(direccion, ''), COALESCE(telefono, ''), COALESCE(email, ''), COALESCE(representante_legal, ''), tipo_contribuyente, creado_en`,
		req.RuccEdula, req.RazonSocial, req.Direccion, req.Telefono, req.Email, req.RepresentanteLegal, req.TipoContribuyente).
		Scan(&c.ID, &c.RuccEdula, &c.RazonSocial, &c.Direccion, &c.Telefono, &c.Email, &c.RepresentanteLegal, &c.TipoContribuyente, &c.CreadoEn)

	if err != nil {
		return nil, fmt.Errorf("error al crear cliente: %w", err)
	}

	return &c, nil
}

func (s *ClienteService) ActualizarCliente(ctx context.Context, id uuid.UUID, req domain.CrearClienteRequest) (*domain.Cliente, error) {
	if s == nil || s.Repo == nil || s.Repo.Pool == nil {
		return nil, fmt.Errorf("base de datos no disponible")
	}

	var c domain.Cliente
	err := s.Repo.Pool.QueryRow(ctx,
		`UPDATE clientes
		 SET ruc_cedula = $1, razon_social = $2, direccion = $3, telefono = $4, email = $5, representante_legal = $6, tipo_contribuyente = $7
		 WHERE id = $8
		 RETURNING id, ruc_cedula, razon_social, COALESCE(direccion, ''), COALESCE(telefono, ''), COALESCE(email, ''), COALESCE(representante_legal, ''), tipo_contribuyente, creado_en`,
		req.RuccEdula, req.RazonSocial, req.Direccion, req.Telefono, req.Email, req.RepresentanteLegal, req.TipoContribuyente, id).
		Scan(&c.ID, &c.RuccEdula, &c.RazonSocial, &c.Direccion, &c.Telefono, &c.Email, &c.RepresentanteLegal, &c.TipoContribuyente, &c.CreadoEn)

	if err != nil {
		return nil, fmt.Errorf("error al actualizar cliente: %w", err)
	}

	return &c, nil
}

func (s *ClienteService) EliminarCliente(ctx context.Context, id uuid.UUID) error {
	if s == nil || s.Repo == nil || s.Repo.Pool == nil {
		return fmt.Errorf("base de datos no disponible")
	}

	cmd, err := s.Repo.Pool.Exec(ctx, `DELETE FROM clientes WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("error al eliminar cliente: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("cliente no encontrado")
	}
	return nil
}
