package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"sifaco/backend/internal/domain"
	"sifaco/backend/internal/repository/postgres"

	"github.com/google/uuid"
)

type UsuarioService struct {
	Repo     *postgres.Repository
	mu       sync.RWMutex
	inMemory []domain.Usuario
}

func NewUsuarioService(repo *postgres.Repository) *UsuarioService {
	s := &UsuarioService{
		Repo:     repo,
		inMemory: make([]domain.Usuario, 0),
	}
	s.seedDefaultUsuarios()
	s.syncDefaultUsuariosToDB()
	return s
}

func (s *UsuarioService) seedDefaultUsuarios() {
	now := time.Now()
	s.inMemory = []domain.Usuario{
		{
			ID:           uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a99"),
			Nombre:       "Administrador General",
			Email:        "admin@sifaco.ni",
			PasswordHash: "admin123",
			Rol:          "ADMIN",
			Activo:       true,
			CreadoEn:     now,
		},
		{
			ID:           uuid.MustParse("b1eebc99-9c0b-4ef8-bb6d-6bb9bd380b00"),
			Nombre:       "Lic. Carlos Mendoza (Contador)",
			Email:        "contador@sifaco.ni",
			PasswordHash: "contador123",
			Rol:          "CONTADOR",
			Activo:       true,
			CreadoEn:     now,
		},
		{
			ID:           uuid.MustParse("c2eebc99-9c0b-4ef8-bb6d-6bb9bd380c01"),
			Nombre:       "María Gutiérrez (Facturación & Caja)",
			Email:        "cajero@sifaco.ni",
			PasswordHash: "cajero123",
			Rol:          "FACTURADOR",
			Activo:       true,
			CreadoEn:     now,
		},
		{
			ID:           uuid.MustParse("d3eebc99-9c0b-4ef8-bb6d-6bb9bd380d02"),
			Nombre:       "Roberto Silva (Gestor CxC)",
			Email:        "cobranza@sifaco.ni",
			PasswordHash: "cobranza123",
			Rol:          "GESTOR_CXC",
			Activo:       true,
			CreadoEn:     now,
		},
		{
			ID:           uuid.MustParse("e4eebc99-9c0b-4ef8-bb6d-6bb9bd380e03"),
			Nombre:       "Elena Ramos (Coordinadora de Eventos)",
			Email:        "eventos@sifaco.ni",
			PasswordHash: "eventos123",
			Rol:          "COORDINADOR_EVENTOS",
			Activo:       true,
			CreadoEn:     now,
		},
	}
}

func (s *UsuarioService) syncDefaultUsuariosToDB() {
	if s.Repo == nil || s.Repo.Pool == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, u := range s.inMemory {
		query := `INSERT INTO usuarios (id, nombre, email, password_hash, rol, activo, creado_en)
				  VALUES ($1, $2, $3, $4, $5, $6, $7)
				  ON CONFLICT (email) DO UPDATE SET 
				      password_hash = EXCLUDED.password_hash,
				      activo = TRUE`
		_, _ = s.Repo.Pool.Exec(ctx, query, u.ID, u.Nombre, u.Email, u.PasswordHash, u.Rol, u.Activo, u.CreadoEn)
	}
}

func (s *UsuarioService) ListarUsuarios(ctx context.Context) ([]domain.Usuario, error) {
	if s == nil || s.Repo == nil || s.Repo.Pool == nil {
		s.mu.RLock()
		defer s.mu.RUnlock()
		resultado := make([]domain.Usuario, len(s.inMemory))
		copy(resultado, s.inMemory)
		return resultado, nil
	}

	query := `SELECT id, nombre, email, rol, activo, creado_en FROM usuarios ORDER BY creado_en DESC`
	rows, err := s.Repo.Pool.Query(ctx, query)
	if err != nil {
		s.mu.RLock()
		defer s.mu.RUnlock()
		resultado := make([]domain.Usuario, len(s.inMemory))
		copy(resultado, s.inMemory)
		return resultado, nil
	}
	defer rows.Close()

	list := make([]domain.Usuario, 0)
	for rows.Next() {
		var u domain.Usuario
		if err := rows.Scan(&u.ID, &u.Nombre, &u.Email, &u.Rol, &u.Activo, &u.CreadoEn); err != nil {
			return nil, fmt.Errorf("error al escanear usuario: %w", err)
		}
		list = append(list, u)
	}
	return list, nil
}

func (s *UsuarioService) ObtenerUsuarioPorID(ctx context.Context, id uuid.UUID) (*domain.Usuario, error) {
	if s == nil || s.Repo == nil || s.Repo.Pool == nil {
		s.mu.RLock()
		defer s.mu.RUnlock()
		for _, u := range s.inMemory {
			if u.ID == id {
				return &u, nil
			}
		}
		return nil, fmt.Errorf("usuario no encontrado con ID %s", id)
	}

	var u domain.Usuario
	query := `SELECT id, nombre, email, rol, activo, creado_en FROM usuarios WHERE id = $1`
	err := s.Repo.Pool.QueryRow(ctx, query, id).Scan(&u.ID, &u.Nombre, &u.Email, &u.Rol, &u.Activo, &u.CreadoEn)
	if err != nil {
		return nil, fmt.Errorf("usuario no encontrado: %w", err)
	}
	return &u, nil
}

func (s *UsuarioService) Autenticar(ctx context.Context, email, password string) (*domain.Usuario, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	emailClean := strings.TrimSpace(email)
	passClean := strings.TrimSpace(password)

	// 1. Buscar en PostgreSQL si está disponible
	if s != nil && s.Repo != nil && s.Repo.Pool != nil {
		var u domain.Usuario
		query := `SELECT id, nombre, email, password_hash, rol, activo, creado_en FROM usuarios WHERE LOWER(email) = LOWER($1)`
		err := s.Repo.Pool.QueryRow(ctx, query, emailClean).Scan(&u.ID, &u.Nombre, &u.Email, &u.PasswordHash, &u.Rol, &u.Activo, &u.CreadoEn)
		if err == nil {
			if !u.Activo {
				return nil, fmt.Errorf("la cuenta del usuario %s está desactivada", emailClean)
			}
			if u.PasswordHash == passClean || u.PasswordHash == "" {
				if u.PasswordHash == "" {
					_, _ = s.Repo.Pool.Exec(ctx, `UPDATE usuarios SET password_hash = $1 WHERE id = $2`, passClean, u.ID)
				}
				return &u, nil
			}
			return nil, fmt.Errorf("contraseña incorrecta")
		}
	}

	// 2. Fallback almacenamiento in-memory
	for _, u := range s.inMemory {
		if strings.EqualFold(u.Email, emailClean) {
			if !u.Activo {
				return nil, fmt.Errorf("la cuenta del usuario %s está desactivada", emailClean)
			}
			if u.PasswordHash == passClean || u.PasswordHash == "" {
				return &u, nil
			}
			return nil, fmt.Errorf("contraseña incorrecta")
		}
	}

	return nil, fmt.Errorf("credenciales no válidas: usuario o contraseña incorrectos")
}

func (s *UsuarioService) CrearUsuario(ctx context.Context, req domain.CrearUsuarioRequest) (*domain.Usuario, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	nuevo := domain.Usuario{
		ID:           uuid.New(),
		Nombre:       req.Nombre,
		Email:        req.Email,
		PasswordHash: req.Password,
		Rol:          req.Rol,
		Activo:       true,
		CreadoEn:     time.Now(),
	}

	s.inMemory = append(s.inMemory, nuevo)

	if s != nil && s.Repo != nil && s.Repo.Pool != nil {
		query := `INSERT INTO usuarios (id, nombre, email, password_hash, rol, activo) VALUES ($1, $2, $3, $4, $5, $6)`
		_, err := s.Repo.Pool.Exec(ctx, query, nuevo.ID, nuevo.Nombre, nuevo.Email, nuevo.PasswordHash, nuevo.Rol, nuevo.Activo)
		if err != nil {
			return nil, fmt.Errorf("error al insertar usuario en base de datos: %w", err)
		}
	}

	return &nuevo, nil
}

func (s *UsuarioService) ActualizarUsuario(ctx context.Context, id uuid.UUID, req domain.ActualizarUsuarioRequest) (*domain.Usuario, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Si la BD está conectada, actualizar directamente en PostgreSQL
	if s != nil && s.Repo != nil && s.Repo.Pool != nil {
		var u domain.Usuario
		query := `UPDATE usuarios SET 
					nombre = COALESCE(NULLIF($1, ''), nombre),
					email = COALESCE(NULLIF($2, ''), email),
					rol = COALESCE(NULLIF($3, ''), rol),
					password_hash = CASE WHEN $4 <> '' THEN $4 ELSE password_hash END,
					activo = COALESCE($5, activo)
				  WHERE id = $6
				  RETURNING id, nombre, email, password_hash, rol, activo, creado_en`

		err := s.Repo.Pool.QueryRow(ctx, query, req.Nombre, req.Email, req.Rol, req.Password, req.Activo, id).Scan(
			&u.ID, &u.Nombre, &u.Email, &u.PasswordHash, &u.Rol, &u.Activo, &u.CreadoEn,
		)

		if err == nil {
			// Sincronizar también la copia in-memory si existe
			for i, mem := range s.inMemory {
				if mem.ID == id {
					s.inMemory[i] = u
					break
				}
			}
			return &u, nil
		}
	}

	// 2. Fallback in-memory
	var index = -1
	for i, u := range s.inMemory {
		if u.ID == id {
			index = i
			break
		}
	}

	if index == -1 {
		return nil, fmt.Errorf("usuario no encontrado para actualización")
	}

	if req.Nombre != "" {
		s.inMemory[index].Nombre = req.Nombre
	}
	if req.Email != "" {
		s.inMemory[index].Email = req.Email
	}
	if req.Rol != "" {
		s.inMemory[index].Rol = req.Rol
	}
	if req.Password != "" {
		s.inMemory[index].PasswordHash = req.Password
	}
	if req.Activo != nil {
		s.inMemory[index].Activo = *req.Activo
	}

	u := s.inMemory[index]
	return &u, nil
}

func (s *UsuarioService) DesactivarUsuario(ctx context.Context, id uuid.UUID) error {
	falso := false
	_, err := s.ActualizarUsuario(ctx, id, domain.ActualizarUsuarioRequest{Activo: &falso})
	return err
}

func (s *UsuarioService) EliminarUsuario(ctx context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s != nil && s.Repo != nil && s.Repo.Pool != nil {
		query := `DELETE FROM usuarios WHERE id = $1`
		_, err := s.Repo.Pool.Exec(ctx, query, id)
		if err != nil {
			return fmt.Errorf("error al eliminar usuario en base de datos: %w", err)
		}
	}

	var index = -1
	for i, u := range s.inMemory {
		if u.ID == id {
			index = i
			break
		}
	}

	if index != -1 {
		s.inMemory = append(s.inMemory[:index], s.inMemory[index+1:]...)
	}

	return nil
}
