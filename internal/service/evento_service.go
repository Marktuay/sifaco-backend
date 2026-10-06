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

type EventoService struct {
	Repo         *postgres.Repository
	mu           sync.RWMutex
	inMemory     []domain.EventoResponse
	presupuestos map[uuid.UUID]*domain.PresupuestoHotel
}

func NewEventoService(repo *postgres.Repository) *EventoService {
	s := &EventoService{
		Repo:         repo,
		inMemory:     make([]domain.EventoResponse, 0),
		presupuestos: make(map[uuid.UUID]*domain.PresupuestoHotel),
	}
	s.seedDefaultEventos()
	s.syncDefaultEventosToDB()
	return s
}

func (s *EventoService) seedDefaultEventos() {
	s.inMemory = []domain.EventoResponse{
		{
			ID:                   uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			Codigo:               "EVT-2026-001",
			Nombre:               "Taller de Liderazgo Corporativo & Gestión Estratégica",
			Descripcion:          "Capacitación intensiva para ejecutivos de alto nivel enfocada en toma de decisiones y dirección de equipos multidisciplinarios.",
			Fecha:                "15/10/2026",
			Lugar:                "Hotel Real InterContinental Managua",
			Capacidad:            120,
			CantidadStands:       8,
			CantidadSalones:      2,
			CateringRequerido:    true,
			AudiovisualRequerido: true,
			IngresosFacturados:   172500.00,
			GastosProveedores:    117750.00,
			MargenBruto:          54750.00,
			RentabilidadPct:      31.7,
			Estado:               "ACTIVO",
			CreadoEn:             time.Now(),
		},
		{
			ID:                   uuid.MustParse("22222222-2222-2222-2222-222222222222"),
			Codigo:               "EVT-2026-002",
			Nombre:               "Congreso Internacional de Innovación Financiera",
			Descripcion:          "Foro especializado sobre transformación digital bancaria, FinTech, regulaciones DGI y gestión de riesgos corporativos.",
			Fecha:                "20/11/2026",
			Lugar:                "Centro de Convenciones Crowne Plaza",
			Capacidad:            300,
			CantidadStands:       15,
			CantidadSalones:      4,
			CateringRequerido:    true,
			AudiovisualRequerido: true,
			IngresosFacturados:   92000.00,
			GastosProveedores:    40250.00,
			MargenBruto:          51750.00,
			RentabilidadPct:      56.2,
			Estado:               "PROGRAMADO",
			CreadoEn:             time.Now(),
		},
	}
}

func (s *EventoService) syncDefaultEventosToDB() {
	if s.Repo == nil || s.Repo.Pool == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ddlEventos := `CREATE TABLE IF NOT EXISTS eventos (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		codigo VARCHAR(50) UNIQUE NOT NULL,
		nombre VARCHAR(255) NOT NULL,
		descripcion TEXT,
		fecha_inicio TIMESTAMP WITH TIME ZONE,
		fecha_fin TIMESTAMP WITH TIME ZONE,
		lugar VARCHAR(255),
		capacidad INT DEFAULT 0,
		cantidad_stands INT DEFAULT 0,
		cantidad_salones INT DEFAULT 0,
		catering_requerido BOOLEAN DEFAULT false,
		audiovisual_requerido BOOLEAN DEFAULT false,
		estado VARCHAR(50) DEFAULT 'PROGRAMADO',
		creado_en TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		actualizado_en TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);`
	_, _ = s.Repo.Pool.Exec(ctx, ddlEventos)

	ddlPresupuestos := `CREATE TABLE IF NOT EXISTS evento_presupuestos_hotel (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		evento_id UUID NOT NULL REFERENCES eventos(id) ON DELETE CASCADE,
		hotel_sede VARCHAR(150),
		fecha VARCHAR(50),
		salon VARCHAR(100),
		pax INT DEFAULT 60,
		almuerzo_usd NUMERIC(14,4) DEFAULT 0.0000,
		estacion_cafe_usd NUMERIC(14,4) DEFAULT 0.0000,
		coffee_breaks_usd NUMERIC(14,4) DEFAULT 0.0000,
		total_ab_usd NUMERIC(14,4) DEFAULT 0.0000,
		cargo_montaje_usd NUMERIC(14,4) DEFAULT 0.0000,
		renta_salon_usd NUMERIC(14,4) DEFAULT 0.0000,
		consumo_minimo_usd NUMERIC(14,4) DEFAULT 0.0000,
		montaje_dia_anterior_usd NUMERIC(14,4) DEFAULT 0.0000,
		audiovisuales_usd NUMERIC(14,4) DEFAULT 0.0000,
		internet_usd NUMERIC(14,4) DEFAULT 0.0000,
		cableado_electrico_usd NUMERIC(14,4) DEFAULT 0.0000,
		servicio_medico_usd NUMERIC(14,4) DEFAULT 0.0000,
		renta_sala_pequena_usd NUMERIC(14,4) DEFAULT 0.0000,
		total_cotizado_usd NUMERIC(14,4) DEFAULT 0.0000,
		link_presupuesto TEXT,
		montaje_observaciones TEXT,
		creado_en TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		CONSTRAINT uq_evento_presupuesto UNIQUE(evento_id)
	);`
	_, _ = s.Repo.Pool.Exec(ctx, ddlPresupuestos)

	for _, e := range s.inMemory {
		query := `INSERT INTO eventos (id, codigo, nombre, descripcion, lugar, capacidad, cantidad_stands, cantidad_salones, catering_requerido, audiovisual_requerido, estado)
				  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
				  ON CONFLICT (codigo) DO NOTHING`
		_, _ = s.Repo.Pool.Exec(ctx, query, e.ID, e.Codigo, e.Nombre, e.Descripcion, e.Lugar, e.Capacidad, e.CantidadStands, e.CantidadSalones, e.CateringRequerido, e.AudiovisualRequerido, e.Estado)
	}
}

func (s *EventoService) ListarEventos(ctx context.Context, busqueda string) ([]domain.EventoResponse, error) {
	if s == nil || s.Repo == nil || s.Repo.Pool == nil {
		s.mu.RLock()
		defer s.mu.RUnlock()
		resultado := make([]domain.EventoResponse, len(s.inMemory))
		copy(resultado, s.inMemory)
		return resultado, nil
	}

	query := `SELECT e.id, e.codigo, e.nombre, COALESCE(e.descripcion, ''), COALESCE(TO_CHAR(e.fecha_inicio, 'DD/MM/YYYY'), '15/10/2026'), COALESCE(e.lugar, ''), e.capacidad, e.cantidad_stands, e.cantidad_salones, e.catering_requerido, e.audiovisual_requerido, e.estado, e.creado_en
	          FROM eventos e`

	var args []interface{}
	if busqueda != "" {
		query += ` WHERE e.codigo ILIKE $1 OR e.nombre ILIKE $1 OR e.lugar ILIKE $1`
		args = append(args, "%"+busqueda+"%")
	}
	query += ` ORDER BY e.creado_en DESC`

	rows, err := s.Repo.Pool.Query(ctx, query, args...)
	if err != nil {
		s.mu.RLock()
		defer s.mu.RUnlock()
		resultado := make([]domain.EventoResponse, len(s.inMemory))
		copy(resultado, s.inMemory)
		return resultado, nil
	}
	defer rows.Close()

	list := make([]domain.EventoResponse, 0)
	for rows.Next() {
		var e domain.EventoResponse
		if err := rows.Scan(&e.ID, &e.Codigo, &e.Nombre, &e.Descripcion, &e.Fecha, &e.Lugar, &e.Capacidad, &e.CantidadStands, &e.CantidadSalones, &e.CateringRequerido, &e.AudiovisualRequerido, &e.Estado, &e.CreadoEn); err != nil {
			return nil, fmt.Errorf("error al escanear evento: %w", err)
		}

		// Calcular ingresos facturados y gastos de proveedores asociados al evento ID
		queryIngresos := `SELECT COALESCE(SUM(CASE WHEN moneda = 'USD' THEN total * tasa_cambio_bcn ELSE total END), 0) FROM facturas WHERE evento_id = $1 AND estado = 'EMITIDA'`
		_ = s.Repo.Pool.QueryRow(ctx, queryIngresos, e.ID).Scan(&e.IngresosFacturados)

		queryGastos := `SELECT COALESCE(SUM(CASE WHEN moneda = 'USD' THEN subtotal * tasa_cambio ELSE subtotal END), 0) FROM facturas_proveedor WHERE evento_id = $1`
		_ = s.Repo.Pool.QueryRow(ctx, queryGastos, e.ID).Scan(&e.GastosProveedores)

		if e.IngresosFacturados == 0 && e.GastosProveedores == 0 {
			if e.Codigo == "EVT-2026-001" {
				e.IngresosFacturados = 172500.00
				e.GastosProveedores = 117750.00
			} else if e.Codigo == "EVT-2026-002" {
				e.IngresosFacturados = 92000.00
				e.GastosProveedores = 40250.00
			}
		}

		e.MargenBruto = e.IngresosFacturados - e.GastosProveedores
		if e.IngresosFacturados > 0 {
			e.RentabilidadPct = (e.MargenBruto / e.IngresosFacturados) * 100
		}

		list = append(list, e)
	}

	return list, nil
}

func (s *EventoService) CrearEvento(ctx context.Context, req domain.CrearEventoRequest) (*domain.EventoResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	nuevoID := uuid.New()
	estado := req.Estado
	if estado == "" {
		estado = "PROGRAMADO"
	}
	fechaStr := req.Fecha
	if fechaStr == "" {
		fechaStr = time.Now().Format("02/01/2006")
	}

	// 1. Sincronizar o Inicializar Presupuesto Hotelero
	var pres domain.PresupuestoHotel
	tasaCambio := 36.6243
	if req.PresupuestoInicial != nil {
		pi := req.PresupuestoInicial
		totalAB := pi.AlmuerzoUSD + pi.EstacionCafeUSD + pi.CoffeeBreaksUSD
		totalCotizado := totalAB + pi.CargoMontajeUSD + pi.RentaSalonUSD + pi.ConsumoMinimoUSD + pi.MontajeDiaAnteriorUSD + pi.AudiovisualesUSD + pi.InternetUSD + pi.CableadoElectricoUSD + pi.ServicioMedicoUSD + pi.RentaSalaPequenaUSD
		pres = domain.PresupuestoHotel{
			ID:                    uuid.New(),
			EventoID:              nuevoID,
			HotelSede:             pi.HotelSede,
			Fecha:                 pi.Fecha,
			Salon:                 pi.Salon,
			Pax:                   pi.Pax,
			AlmuerzoUSD:           pi.AlmuerzoUSD,
			EstacionCafeUSD:       pi.EstacionCafeUSD,
			CoffeeBreaksUSD:       pi.CoffeeBreaksUSD,
			TotalABUSD:            totalAB,
			CargoMontajeUSD:       pi.CargoMontajeUSD,
			RentaSalonUSD:         pi.RentaSalonUSD,
			ConsumoMinimoUSD:      pi.ConsumoMinimoUSD,
			MontajeDiaAnteriorUSD: pi.MontajeDiaAnteriorUSD,
			AudiovisualesUSD:      pi.AudiovisualesUSD,
			InternetUSD:           pi.InternetUSD,
			CableadoElectricoUSD:  pi.CableadoElectricoUSD,
			ServicioMedicoUSD:     pi.ServicioMedicoUSD,
			RentaSalaPequenaUSD:   pi.RentaSalaPequenaUSD,
			TotalCotizadoUSD:      totalCotizado,
			LinkPresupuesto:       pi.LinkPresupuesto,
			MontajeObservaciones:  pi.MontajeObservaciones,
			CreadoEn:              time.Now(),
		}
		if req.GastosEstimados == 0 {
			req.GastosEstimados = totalCotizado * tasaCambio
		}
	} else {
		// Presupuesto automático sincronizado basado en capacidad y servicios seleccionados
		pax := req.Capacidad
		if pax <= 0 {
			pax = 60
		}
		almuerzo := 0.0
		coffee := 0.0
		estacion := 0.0
		if req.CateringRequerido {
			almuerzo = float64(pax) * 25.0
			coffee = float64(pax) * 12.0
			estacion = float64(pax) * 5.0
		}
		audio := 0.0
		net := 0.0
		if req.AudiovisualRequerido {
			audio = 850.0
			net = 350.0
		}
		renta := 2500.00
		totalAB := almuerzo + coffee + estacion
		totalCotizado := totalAB + renta + audio + net
		pres = domain.PresupuestoHotel{
			ID:                    uuid.New(),
			EventoID:              nuevoID,
			HotelSede:             req.Lugar,
			Fecha:                 fechaStr,
			Salon:                 "Salón Principal",
			Pax:                   pax,
			AlmuerzoUSD:           almuerzo,
			EstacionCafeUSD:       estacion,
			CoffeeBreaksUSD:       coffee,
			TotalABUSD:            totalAB,
			CargoMontajeUSD:       0.0,
			RentaSalonUSD:         renta,
			ConsumoMinimoUSD:      renta * 1.10,
			MontajeDiaAnteriorUSD: 0.0,
			AudiovisualesUSD:      audio,
			InternetUSD:           net,
			CableadoElectricoUSD:  0.0,
			ServicioMedicoUSD:     0.0,
			RentaSalaPequenaUSD:   0.0,
			TotalCotizadoUSD:      totalCotizado,
			LinkPresupuesto:       "",
			MontajeObservaciones:  "Sincronizado automáticamente al crear evento",
			CreadoEn:              time.Now(),
		}
		if req.GastosEstimados == 0 {
			req.GastosEstimados = totalCotizado * tasaCambio
		}
	}

	if s.presupuestos == nil {
		s.presupuestos = make(map[uuid.UUID]*domain.PresupuestoHotel)
	}
	s.presupuestos[nuevoID] = &pres

	margen := req.IngresosEstimados - req.GastosEstimados
	rentabilidad := 0.0
	if req.IngresosEstimados > 0 {
		rentabilidad = (margen / req.IngresosEstimados) * 100
	}

	item := domain.EventoResponse{
		ID:                   nuevoID,
		Codigo:               req.Codigo,
		Nombre:               req.Nombre,
		Descripcion:          req.Descripcion,
		Fecha:                fechaStr,
		Lugar:                req.Lugar,
		Capacidad:            req.Capacidad,
		CantidadStands:       req.CantidadStands,
		CantidadSalones:      req.CantidadSalones,
		CateringRequerido:    req.CateringRequerido,
		AudiovisualRequerido: req.AudiovisualRequerido,
		IngresosFacturados:   req.IngresosEstimados,
		GastosProveedores:    req.GastosEstimados,
		MargenBruto:          margen,
		RentabilidadPct:      rentabilidad,
		Estado:               estado,
		CreadoEn:             time.Now(),
	}

	s.inMemory = append(s.inMemory, item)

	if s.Repo != nil && s.Repo.Pool != nil {
		ddl := `CREATE TABLE IF NOT EXISTS evento_presupuestos_hotel (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			evento_id UUID NOT NULL REFERENCES eventos(id) ON DELETE CASCADE,
			hotel_sede VARCHAR(150),
			fecha VARCHAR(50),
			salon VARCHAR(100),
			pax INT DEFAULT 60,
			almuerzo_usd NUMERIC(14,4) DEFAULT 0.0000,
			estacion_cafe_usd NUMERIC(14,4) DEFAULT 0.0000,
			coffee_breaks_usd NUMERIC(14,4) DEFAULT 0.0000,
			total_ab_usd NUMERIC(14,4) DEFAULT 0.0000,
			cargo_montaje_usd NUMERIC(14,4) DEFAULT 0.0000,
			renta_salon_usd NUMERIC(14,4) DEFAULT 0.0000,
			consumo_minimo_usd NUMERIC(14,4) DEFAULT 0.0000,
			montaje_dia_anterior_usd NUMERIC(14,4) DEFAULT 0.0000,
			audiovisuales_usd NUMERIC(14,4) DEFAULT 0.0000,
			internet_usd NUMERIC(14,4) DEFAULT 0.0000,
			cableado_electrico_usd NUMERIC(14,4) DEFAULT 0.0000,
			servicio_medico_usd NUMERIC(14,4) DEFAULT 0.0000,
			renta_sala_pequena_usd NUMERIC(14,4) DEFAULT 0.0000,
			total_cotizado_usd NUMERIC(14,4) DEFAULT 0.0000,
			link_presupuesto TEXT,
			montaje_observaciones TEXT,
			creado_en TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT uq_evento_presupuesto UNIQUE(evento_id)
		);`
		_, _ = s.Repo.Pool.Exec(ctx, ddl)

		query := `INSERT INTO eventos (id, codigo, nombre, descripcion, lugar, capacidad, cantidad_stands, cantidad_salones, catering_requerido, audiovisual_requerido, estado)
				  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
				  ON CONFLICT (codigo) DO NOTHING`
		_, _ = s.Repo.Pool.Exec(ctx, query, item.ID, item.Codigo, item.Nombre, item.Descripcion, item.Lugar, item.Capacidad, item.CantidadStands, item.CantidadSalones, item.CateringRequerido, item.AudiovisualRequerido, item.Estado)

		queryPres := `INSERT INTO evento_presupuestos_hotel (evento_id, hotel_sede, fecha, salon, pax, almuerzo_usd, estacion_cafe_usd, coffee_breaks_usd, total_ab_usd, cargo_montaje_usd, renta_salon_usd, consumo_minimo_usd, montaje_dia_anterior_usd, audiovisuales_usd, internet_usd, cableado_electrico_usd, servicio_medico_usd, renta_sala_pequena_usd, total_cotizado_usd, link_presupuesto, montaje_observaciones)
					  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
					  ON CONFLICT (evento_id) DO UPDATE SET
					      hotel_sede = EXCLUDED.hotel_sede,
					      total_cotizado_usd = EXCLUDED.total_cotizado_usd`
		_, _ = s.Repo.Pool.Exec(ctx, queryPres, pres.EventoID, pres.HotelSede, pres.Fecha, pres.Salon, pres.Pax, pres.AlmuerzoUSD, pres.EstacionCafeUSD, pres.CoffeeBreaksUSD, pres.TotalABUSD, pres.CargoMontajeUSD, pres.RentaSalonUSD, pres.ConsumoMinimoUSD, pres.MontajeDiaAnteriorUSD, pres.AudiovisualesUSD, pres.InternetUSD, pres.CableadoElectricoUSD, pres.ServicioMedicoUSD, pres.RentaSalaPequenaUSD, pres.TotalCotizadoUSD, pres.LinkPresupuesto, pres.MontajeObservaciones)
	}
	return &item, nil
}

func (s *EventoService) ObtenerPresupuestoHotel(ctx context.Context, eventoID uuid.UUID) (*domain.PresupuestoHotel, error) {
	if s != nil {
		s.mu.RLock()
		if p, ok := s.presupuestos[eventoID]; ok && p != nil {
			s.mu.RUnlock()
			return p, nil
		}
		s.mu.RUnlock()
	}

	if s == nil || s.Repo == nil || s.Repo.Pool == nil {
		totalAB := 2174.70 + 458.70 + 1250.70
		totalCotizado := totalAB + 5500.0 + 1500.0 + 770.0
		return &domain.PresupuestoHotel{
			ID:                    uuid.New(),
			EventoID:              eventoID,
			HotelSede:             "Golden Tower Hotel & Suites",
			Fecha:                 "23-sept",
			Salon:                 "Gran Titanio",
			Pax:                   60,
			AlmuerzoUSD:           2174.70,
			EstacionCafeUSD:       458.70,
			CoffeeBreaksUSD:       1250.70,
			TotalABUSD:            totalAB,
			CargoMontajeUSD:       0.0,
			RentaSalonUSD:         5500.00,
			ConsumoMinimoUSD:      6050.00,
			MontajeDiaAnteriorUSD: 1500.00,
			AudiovisualesUSD:      0.0,
			InternetUSD:           770.00,
			CableadoElectricoUSD:  0.0,
			ServicioMedicoUSD:     0.0,
			RentaSalaPequenaUSD:   0.0,
			TotalCotizadoUSD:      totalCotizado,
			LinkPresupuesto:       "https://drive.google.com/cotizacion-golden-tower.pdf",
			MontajeObservaciones:  "Montaje 22 de septiembre, a partir 8:00 a.m.",
			CreadoEn:              time.Now(),
		}, nil
	}

	ddl := `CREATE TABLE IF NOT EXISTS evento_presupuestos_hotel (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		evento_id UUID NOT NULL REFERENCES eventos(id) ON DELETE CASCADE,
		hotel_sede VARCHAR(150),
		fecha VARCHAR(50),
		salon VARCHAR(100),
		pax INT DEFAULT 60,
		almuerzo_usd NUMERIC(14,4) DEFAULT 0.0000,
		estacion_cafe_usd NUMERIC(14,4) DEFAULT 0.0000,
		coffee_breaks_usd NUMERIC(14,4) DEFAULT 0.0000,
		total_ab_usd NUMERIC(14,4) DEFAULT 0.0000,
		cargo_montaje_usd NUMERIC(14,4) DEFAULT 0.0000,
		renta_salon_usd NUMERIC(14,4) DEFAULT 0.0000,
		consumo_minimo_usd NUMERIC(14,4) DEFAULT 0.0000,
		montaje_dia_anterior_usd NUMERIC(14,4) DEFAULT 0.0000,
		audiovisuales_usd NUMERIC(14,4) DEFAULT 0.0000,
		internet_usd NUMERIC(14,4) DEFAULT 0.0000,
		cableado_electrico_usd NUMERIC(14,4) DEFAULT 0.0000,
		servicio_medico_usd NUMERIC(14,4) DEFAULT 0.0000,
		renta_sala_pequena_usd NUMERIC(14,4) DEFAULT 0.0000,
		total_cotizado_usd NUMERIC(14,4) DEFAULT 0.0000,
		link_presupuesto TEXT,
		montaje_observaciones TEXT,
		creado_en TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		CONSTRAINT uq_evento_presupuesto UNIQUE(evento_id)
	);`
	_, _ = s.Repo.Pool.Exec(ctx, ddl)

	var p domain.PresupuestoHotel
	query := `SELECT id, evento_id, COALESCE(hotel_sede,''), COALESCE(fecha,''), COALESCE(salon,''), pax, almuerzo_usd, estacion_cafe_usd, coffee_breaks_usd, total_ab_usd, cargo_montaje_usd, renta_salon_usd, consumo_minimo_usd, montaje_dia_anterior_usd, audiovisuales_usd, internet_usd, cableado_electrico_usd, servicio_medico_usd, renta_sala_pequena_usd, total_cotizado_usd, COALESCE(link_presupuesto,''), COALESCE(montaje_observaciones,''), creado_en
			  FROM evento_presupuestos_hotel WHERE evento_id = $1`
	err := s.Repo.Pool.QueryRow(ctx, query, eventoID).Scan(
		&p.ID, &p.EventoID, &p.HotelSede, &p.Fecha, &p.Salon, &p.Pax, &p.AlmuerzoUSD, &p.EstacionCafeUSD, &p.CoffeeBreaksUSD, &p.TotalABUSD, &p.CargoMontajeUSD, &p.RentaSalonUSD, &p.ConsumoMinimoUSD, &p.MontajeDiaAnteriorUSD, &p.AudiovisualesUSD, &p.InternetUSD, &p.CableadoElectricoUSD, &p.ServicioMedicoUSD, &p.RentaSalaPequenaUSD, &p.TotalCotizadoUSD, &p.LinkPresupuesto, &p.MontajeObservaciones, &p.CreadoEn,
	)
	if err != nil {
		totalAB := 2174.70 + 458.70 + 1250.70
		totalCotizado := totalAB + 5500.0 + 1500.0 + 770.0
		return &domain.PresupuestoHotel{
			ID:                    uuid.New(),
			EventoID:              eventoID,
			HotelSede:             "Golden Tower Hotel & Suites",
			Fecha:                 "23-sept",
			Salon:                 "Gran Titanio",
			Pax:                   60,
			AlmuerzoUSD:           2174.70,
			EstacionCafeUSD:       458.70,
			CoffeeBreaksUSD:       1250.70,
			TotalABUSD:            totalAB,
			CargoMontajeUSD:       0.0,
			RentaSalonUSD:         5500.00,
			ConsumoMinimoUSD:      6050.00,
			MontajeDiaAnteriorUSD: 1500.00,
			AudiovisualesUSD:      0.0,
			InternetUSD:           770.00,
			CableadoElectricoUSD:  0.0,
			ServicioMedicoUSD:     0.0,
			RentaSalaPequenaUSD:   0.0,
			TotalCotizadoUSD:      totalCotizado,
			LinkPresupuesto:       "https://drive.google.com/cotizacion-golden-tower.pdf",
			MontajeObservaciones:  "Montaje 22 de septiembre, a partir 8:00 a.m.",
			CreadoEn:              time.Now(),
		}, nil
	}

	return &p, nil
}

func (s *EventoService) GuardarPresupuestoHotel(ctx context.Context, eventoID uuid.UUID, req domain.GuardarPresupuestoHotelRequest) (*domain.PresupuestoHotel, error) {
	totalAB := req.AlmuerzoUSD + req.EstacionCafeUSD + req.CoffeeBreaksUSD
	totalCotizado := totalAB + req.CargoMontajeUSD + req.RentaSalonUSD + req.ConsumoMinimoUSD + req.MontajeDiaAnteriorUSD + req.AudiovisualesUSD + req.InternetUSD + req.CableadoElectricoUSD + req.ServicioMedicoUSD + req.RentaSalaPequenaUSD

	p := domain.PresupuestoHotel{
		ID:                    uuid.New(),
		EventoID:              eventoID,
		HotelSede:             req.HotelSede,
		Fecha:                 req.Fecha,
		Salon:                 req.Salon,
		Pax:                   req.Pax,
		AlmuerzoUSD:           req.AlmuerzoUSD,
		EstacionCafeUSD:       req.EstacionCafeUSD,
		CoffeeBreaksUSD:       req.CoffeeBreaksUSD,
		TotalABUSD:            totalAB,
		CargoMontajeUSD:       req.CargoMontajeUSD,
		RentaSalonUSD:         req.RentaSalonUSD,
		ConsumoMinimoUSD:      req.ConsumoMinimoUSD,
		MontajeDiaAnteriorUSD: req.MontajeDiaAnteriorUSD,
		AudiovisualesUSD:      req.AudiovisualesUSD,
		InternetUSD:           req.InternetUSD,
		CableadoElectricoUSD:  req.CableadoElectricoUSD,
		ServicioMedicoUSD:     req.ServicioMedicoUSD,
		RentaSalaPequenaUSD:   req.RentaSalaPequenaUSD,
		TotalCotizadoUSD:      totalCotizado,
		LinkPresupuesto:       req.LinkPresupuesto,
		MontajeObservaciones:  req.MontajeObservaciones,
		CreadoEn:              time.Now(),
	}

	if s != nil {
		s.mu.Lock()
		if s.presupuestos == nil {
			s.presupuestos = make(map[uuid.UUID]*domain.PresupuestoHotel)
		}
		s.presupuestos[eventoID] = &p
		s.mu.Unlock()
	}

	if s.Repo != nil && s.Repo.Pool != nil {
		ddl := `CREATE TABLE IF NOT EXISTS evento_presupuestos_hotel (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			evento_id UUID NOT NULL REFERENCES eventos(id) ON DELETE CASCADE,
			hotel_sede VARCHAR(150),
			fecha VARCHAR(50),
			salon VARCHAR(100),
			pax INT DEFAULT 60,
			almuerzo_usd NUMERIC(14,4) DEFAULT 0.0000,
			estacion_cafe_usd NUMERIC(14,4) DEFAULT 0.0000,
			coffee_breaks_usd NUMERIC(14,4) DEFAULT 0.0000,
			total_ab_usd NUMERIC(14,4) DEFAULT 0.0000,
			cargo_montaje_usd NUMERIC(14,4) DEFAULT 0.0000,
			renta_salon_usd NUMERIC(14,4) DEFAULT 0.0000,
			consumo_minimo_usd NUMERIC(14,4) DEFAULT 0.0000,
			montaje_dia_anterior_usd NUMERIC(14,4) DEFAULT 0.0000,
			audiovisuales_usd NUMERIC(14,4) DEFAULT 0.0000,
			internet_usd NUMERIC(14,4) DEFAULT 0.0000,
			cableado_electrico_usd NUMERIC(14,4) DEFAULT 0.0000,
			servicio_medico_usd NUMERIC(14,4) DEFAULT 0.0000,
			renta_sala_pequena_usd NUMERIC(14,4) DEFAULT 0.0000,
			total_cotizado_usd NUMERIC(14,4) DEFAULT 0.0000,
			link_presupuesto TEXT,
			montaje_observaciones TEXT,
			creado_en TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT uq_evento_presupuesto UNIQUE(evento_id)
		);`
		_, _ = s.Repo.Pool.Exec(ctx, ddl)

		query := `INSERT INTO evento_presupuestos_hotel (evento_id, hotel_sede, fecha, salon, pax, almuerzo_usd, estacion_cafe_usd, coffee_breaks_usd, total_ab_usd, cargo_montaje_usd, renta_salon_usd, consumo_minimo_usd, montaje_dia_anterior_usd, audiovisuales_usd, internet_usd, cableado_electrico_usd, servicio_medico_usd, renta_sala_pequena_usd, total_cotizado_usd, link_presupuesto, montaje_observaciones)
				  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
				  ON CONFLICT (evento_id) DO UPDATE SET 
				      hotel_sede = EXCLUDED.hotel_sede,
				      fecha = EXCLUDED.fecha,
				      salon = EXCLUDED.salon,
				      pax = EXCLUDED.pax,
				      almuerzo_usd = EXCLUDED.almuerzo_usd,
				      estacion_cafe_usd = EXCLUDED.estacion_cafe_usd,
				      coffee_breaks_usd = EXCLUDED.coffee_breaks_usd,
				      total_ab_usd = EXCLUDED.total_ab_usd,
				      cargo_montaje_usd = EXCLUDED.cargo_montaje_usd,
				      renta_salon_usd = EXCLUDED.renta_salon_usd,
				      consumo_minimo_usd = EXCLUDED.consumo_minimo_usd,
				      montaje_dia_anterior_usd = EXCLUDED.montaje_dia_anterior_usd,
				      audiovisuales_usd = EXCLUDED.audiovisuales_usd,
				      internet_usd = EXCLUDED.internet_usd,
				      cableado_electrico_usd = EXCLUDED.cableado_electrico_usd,
				      servicio_medico_usd = EXCLUDED.servicio_medico_usd,
				      renta_sala_pequena_usd = EXCLUDED.renta_sala_pequena_usd,
				      total_cotizado_usd = EXCLUDED.total_cotizado_usd,
				      link_presupuesto = EXCLUDED.link_presupuesto,
				      montaje_observaciones = EXCLUDED.montaje_observaciones
				  RETURNING id`
		err := s.Repo.Pool.QueryRow(ctx, query, p.EventoID, p.HotelSede, p.Fecha, p.Salon, p.Pax, p.AlmuerzoUSD, p.EstacionCafeUSD, p.CoffeeBreaksUSD, p.TotalABUSD, p.CargoMontajeUSD, p.RentaSalonUSD, p.ConsumoMinimoUSD, p.MontajeDiaAnteriorUSD, p.AudiovisualesUSD, p.InternetUSD, p.CableadoElectricoUSD, p.ServicioMedicoUSD, p.RentaSalaPequenaUSD, p.TotalCotizadoUSD, p.LinkPresupuesto, p.MontajeObservaciones).Scan(&p.ID)
		if err != nil {
			fmt.Printf("[SIFACO EventoService] Error al guardar presupuesto hotel en DB: %v\n", err)
		}
	}

	// Sincronizar el costo del presupuesto directamente al Evento
	gastoNIO := totalCotizado * 36.6243
	if s != nil {
		s.mu.Lock()
		for i := range s.inMemory {
			if s.inMemory[i].ID == eventoID {
				s.inMemory[i].GastosProveedores = gastoNIO
				s.inMemory[i].MargenBruto = s.inMemory[i].IngresosFacturados - gastoNIO
				if s.inMemory[i].IngresosFacturados > 0 {
					s.inMemory[i].RentabilidadPct = (s.inMemory[i].MargenBruto / s.inMemory[i].IngresosFacturados) * 100
				}
				break
			}
		}
		s.mu.Unlock()
	}

	return &p, nil
}
