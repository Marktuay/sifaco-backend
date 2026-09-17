package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"sifaco/backend/internal/domain"
	"sifaco/backend/internal/repository/postgres"

	"github.com/google/uuid"
)

type AuditoriaService struct {
	Repo      *postgres.Repository
	mu        sync.RWMutex
	inMemory  []domain.AuditoriaLogItem
}

func NewAuditoriaService(repo *postgres.Repository) *AuditoriaService {
	s := &AuditoriaService{
		Repo:     repo,
		inMemory: make([]domain.AuditoriaLogItem, 0),
	}
	s.seedDefaultLogs()
	return s
}

func (s *AuditoriaService) seedDefaultLogs() {
	s.inMemory = append(s.inMemory,
		domain.AuditoriaLogItem{
			ID:            uuid.MustParse("00000000-0000-0000-0000-000000000001"),
			UsuarioNombre: "Administrador General",
			UsuarioEmail:  "admin@sifaco.ni",
			Rol:           "ADMIN",
			Accion:        "INICIO_SESION",
			TablaAfectada: "sesiones",
			Detalles:      "Inicio de sesión exitoso en el sistema SIFACO",
			IPOrigen:      "127.0.0.1",
			FechaHora:     time.Now().Add(-2 * time.Hour).Format("02/01/2006 15:04:05"),
			CreadoEn:      time.Now().Add(-2 * time.Hour),
		},
		domain.AuditoriaLogItem{
			ID:            uuid.MustParse("00000000-0000-0000-0000-000000000002"),
			UsuarioNombre: "Administrador General",
			UsuarioEmail:  "admin@sifaco.ni",
			Rol:           "ADMIN",
			Accion:        "EMISION_FACTURA_PREIMPRESA",
			TablaAfectada: "facturas",
			Detalles:      "Factura A-000001 emitida por C$ 50,000.00 con bloqueo DGI FOR UPDATE",
			IPOrigen:      "127.0.0.1",
			FechaHora:     time.Now().Add(-1 * time.Hour).Format("02/01/2006 15:04:05"),
			CreadoEn:      time.Now().Add(-1 * time.Hour),
		},
	)
}

func (s *AuditoriaService) ListarLogs(ctx context.Context) ([]domain.AuditoriaLogItem, error) {
	if s == nil || s.Repo == nil || s.Repo.Pool == nil {
		s.mu.RLock()
		defer s.mu.RUnlock()
		// Copia inversa (últimos eventos primero)
		resultado := make([]domain.AuditoriaLogItem, len(s.inMemory))
		for i, item := range s.inMemory {
			resultado[len(s.inMemory)-1-i] = item
		}
		return resultado, nil
	}

	query := `
		SELECT 
			a.id,
			COALESCE(u.nombre, 'Usuario Sistema'),
			COALESCE(u.email, 'admin@sifaco.ni'),
			COALESCE(u.rol, 'ADMIN'),
			a.accion,
			a.tabla_afectada,
			COALESCE(a.valores_nuevos->>'detalles', 'Operación registrada'),
			COALESCE(a.ip_origen, '127.0.0.1'),
			TO_CHAR(a.creado_en, 'DD/MM/YYYY HH24:MI:SS') as fecha_hora,
			a.creado_en
		FROM auditoria_logs a
		LEFT JOIN usuarios u ON u.id = a.usuario_id
		ORDER BY a.creado_en DESC
		LIMIT 100
	`

	rows, err := s.Repo.Pool.Query(ctx, query)
	if err != nil {
		s.mu.RLock()
		defer s.mu.RUnlock()
		resultado := make([]domain.AuditoriaLogItem, len(s.inMemory))
		for i, item := range s.inMemory {
			resultado[len(s.inMemory)-1-i] = item
		}
		return resultado, nil
	}
	defer rows.Close()

	var logs []domain.AuditoriaLogItem
	for rows.Next() {
		var item domain.AuditoriaLogItem
		err := rows.Scan(&item.ID, &item.UsuarioNombre, &item.UsuarioEmail, &item.Rol, &item.Accion,
			&item.TablaAfectada, &item.Detalles, &item.IPOrigen, &item.FechaHora, &item.CreadoEn)
		if err != nil {
			return nil, fmt.Errorf("error al escanear log de auditoría: %w", err)
		}
		logs = append(logs, item)
	}

	return logs, nil
}

func (s *AuditoriaService) RegistrarLog(ctx context.Context, req domain.RegistrarAuditoriaRequest, clientIP string) (*domain.AuditoriaLogItem, error) {
	if clientIP == "" {
		clientIP = "127.0.0.1"
	}

	nombre := "Administrador General"
	if req.UsuarioEmail != "" {
		nombre = "Usuario (" + req.UsuarioEmail + ")"
	}
	rol := req.Rol
	if rol == "" {
		rol = "ADMIN"
	}

	item := domain.AuditoriaLogItem{
		ID:            uuid.New(),
		UsuarioNombre: nombre,
		UsuarioEmail:  req.UsuarioEmail,
		Rol:           rol,
		Accion:        req.Accion,
		TablaAfectada: req.TablaAfectada,
		Detalles:      req.Detalles,
		IPOrigen:      clientIP,
		FechaHora:     time.Now().Format("02/01/2006 15:04:05"),
		CreadoEn:      time.Now(),
	}

	s.mu.Lock()
	s.inMemory = append(s.inMemory, item)
	s.mu.Unlock()

	if s != nil && s.Repo != nil && s.Repo.Pool != nil {
		detallesJSON := fmt.Sprintf(`{"detalles": "%s"}`, req.Detalles)
		_, _ = s.Repo.Pool.Exec(ctx,
			`INSERT INTO auditoria_logs (accion, tabla_afectada, valores_nuevos, ip_origen) VALUES ($1, $2, $3::jsonb, $4)`,
			req.Accion, req.TablaAfectada, detallesJSON, clientIP)
	}

	return &item, nil
}
