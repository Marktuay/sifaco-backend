package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"math"
	"time"

	"sifaco/backend/internal/domain"
	"sifaco/backend/internal/repository/postgres"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type FacturacionService struct {
	Repo *postgres.Repository
}

func NewFacturacionService(repo *postgres.Repository) *FacturacionService {
	s := &FacturacionService{Repo: repo}
	s.syncDefaultTalonarioToDB()
	return s
}

func (s *FacturacionService) syncDefaultTalonarioToDB() {
	if s.Repo == nil || s.Repo.Pool == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	talonarioID := uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380f01")
	fechaVence := time.Now().AddDate(2, 0, 0)

	query := `INSERT INTO talonarios_dgi (id, numero_autorizacion, serie, numero_desde, numero_hasta, numero_actual, fecha_vencimiento, activo)
			  VALUES ($1, $2, $3, $4, $5, $6, $7, TRUE)
			  ON CONFLICT DO NOTHING`
	_, _ = s.Repo.Pool.Exec(ctx, query, talonarioID, "DGI-2026-8899", "A", 1, 1000, 1, fechaVence)
}

// GenerarFacturaPreimpresa ejecuta la transacción atómica fiscal DGI para emitir una factura preimpresa
func (s *FacturacionService) GenerarFacturaPreimpresa(ctx context.Context, req domain.CrearFacturaRequest) (*domain.CrearFacturaResponse, error) {
	if s == nil || s.Repo == nil || s.Repo.Pool == nil {
		return nil, fmt.Errorf("base de datos no disponible")
	}

	var resp domain.CrearFacturaResponse
	var cliente domain.Cliente

	err := s.Repo.ExecTx(ctx, func(tx pgx.Tx) error {
		// 1. Obtener Cliente
		err := tx.QueryRow(ctx,
			`SELECT id, ruc_cedula, razon_social, COALESCE(direccion, ''), COALESCE(telefono, ''), COALESCE(email, ''), COALESCE(representante_legal, ''), tipo_contribuyente 
			 FROM clientes WHERE id = $1`, req.ClienteID).
			Scan(&cliente.ID, &cliente.RuccEdula, &cliente.RazonSocial, &cliente.Direccion, &cliente.Telefono, &cliente.Email, &cliente.RepresentanteLegal, &cliente.TipoContribuyente)
		if err != nil {
			return fmt.Errorf("cliente no encontrado: %w", err)
		}

		// 2. BLOQUEO PESIMISTA ATÓMICO SELECT ... FOR UPDATE sobre talonario DGI activo
		var talonario domain.TalonarioDGI
		err = tx.QueryRow(ctx,
			`SELECT id, numero_autorizacion, serie, numero_desde, numero_hasta, numero_actual, fecha_vencimiento, activo
			 FROM talonarios_dgi
			 WHERE activo = TRUE
			 FOR UPDATE`).
			Scan(&talonario.ID, &talonario.NumeroAutorizacion, &talonario.Serie, &talonario.NumeroDesde, &talonario.NumeroHasta, &talonario.NumeroActual, &talonario.FechaVencimiento, &talonario.Activo)

		if err != nil {
			return fmt.Errorf("error fiscal DGI: No hay ningún talonario preimpreso activo disponible: %w", err)
		}

		// 3. Validaciones Fiscales DGI (Vencimiento y Límite de Folios)
		ahora := time.Now()
		if ahora.After(talonario.FechaVencimiento) {
			return fmt.Errorf("ERROR FISCAL DGI: El talonario autorización %s venció el %s. Transacción cancelada",
				talonario.NumeroAutorizacion, talonario.FechaVencimiento.Format("02/01/2006"))
		}

		if talonario.NumeroActual > talonario.NumeroHasta {
			return fmt.Errorf("ERROR FISCAL DGI: El talonario autorización %s ha agotado su rango autorizado (%d a %d)",
				talonario.NumeroAutorizacion, talonario.NumeroDesde, talonario.NumeroHasta)
		}

		// 4. Generación del correlativo preimpreso (ej. A-000001)
		correlativoStr := fmt.Sprintf("%s-%06d", talonario.Serie, talonario.NumeroActual)
		folioAsignado := talonario.NumeroActual

		// Incremento atómico del folio actual en el talonario
		_, err = tx.Exec(ctx, `UPDATE talonarios_dgi SET numero_actual = numero_actual + 1 WHERE id = $1`, talonario.ID)
		if err != nil {
			return fmt.Errorf("error al actualizar correlativo de talonario DGI: %w", err)
		}

		// 5. Cálculo de Subtotales e Impuestos (IVA 15%)
		var subtotalGravado, subtotalExento, totalIVA float64
		var detallesInsert []domain.FacturaDetalle

		for _, item := range req.Detalles {
			subtotalLinea := math.Round((item.Cantidad*item.PrecioUnitario)*10000) / 10000
			var ivaLinea float64
			if !item.Exento {
				ivaLinea = math.Round((subtotalLinea*0.15)*10000) / 10000
				subtotalGravado += subtotalLinea
				totalIVA += ivaLinea
			} else {
				subtotalExento += subtotalLinea
			}
			totalLinea := subtotalLinea + ivaLinea

			detallesInsert = append(detallesInsert, domain.FacturaDetalle{
				ID:             uuid.New(),
				Descripcion:    item.Descripcion,
				Cantidad:       item.Cantidad,
				PrecioUnitario: item.PrecioUnitario,
				Exento:         item.Exento,
				Subtotal:       subtotalLinea,
				MontoIVA:       ivaLinea,
				TotalLinea:     totalLinea,
			})
		}

		totalFactura := subtotalGravado + subtotalExento + totalIVA

		// 6. Validación de Cuotas si es Venta al Crédito
		var cuotasInsert []domain.FacturaCuota
		if req.TipoPago == "CREDITO" {
			if len(req.Cuotas) == 0 {
				return fmt.Errorf("debe especificar al menos una cuota para ventas al crédito")
			}

			var sumaCuotas float64
			for _, c := range req.Cuotas {
				sumaCuotas += c.Monto
			}

			// Validar igualdad estricta de suma de cuotas con total de factura
			if math.Abs(sumaCuotas-totalFactura) > 0.01 {
				return fmt.Errorf("error en plan de pagos: La suma de cuotas (%.2f) no coincide con el total de la factura (%.2f)",
					sumaCuotas, totalFactura)
			}

			for _, c := range req.Cuotas {
				cuotasInsert = append(cuotasInsert, domain.FacturaCuota{
					ID:          uuid.New(),
					NumeroCuota: c.NumeroCuota,
					FechaVence:  c.FechaVence,
					Monto:       c.Monto,
					Saldo:       c.Monto,
					Estado:      "PENDIENTE",
				})
			}
		}

		// 7. Insertar Cabecera de Factura con Conversión de Monto a Letras
		facturaID := uuid.New()
		montoLetrasStr := domain.NumeroALetras(totalFactura, req.Moneda)

		factura := domain.Factura{
			ID:                    facturaID,
			TalonarioID:           talonario.ID,
			CorrelativoPreimpreso: correlativoStr,
			Serie:                 talonario.Serie,
			NumeroFolio:           folioAsignado,
			ClienteID:             req.ClienteID,
			EventoID:              req.EventoID,
			FechaEmision:          time.Now(),
			TipoPago:              req.TipoPago,
			Moneda:                req.Moneda,
			TasaCambioBCN:         req.TasaCambioBCN,
			SubtotalGravado:       subtotalGravado,
			SubtotalExento:        subtotalExento,
			MontoIVA:              totalIVA,
			Total:                 totalFactura,
			SaldoPendiente:        totalFactura,
			Estado:                "EMITIDA",
			Observaciones:         req.Observaciones,
			CreadoEn:              time.Now(),
		}

		_, err = tx.Exec(ctx,
			`INSERT INTO facturas (id, talonario_id, correlativo_preimpreso, serie, numero_folio, cliente_id, evento_id, fecha_emision, tipo_pago, moneda, tasa_cambio_bcn, subtotal_gravado, subtotal_exento, monto_iva, total, monto_letras, saldo_pendiente, estado, observaciones)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)`,
			factura.ID, factura.TalonarioID, factura.CorrelativoPreimpreso, factura.Serie, factura.NumeroFolio,
			factura.ClienteID, factura.EventoID, factura.FechaEmision, factura.TipoPago, factura.Moneda,
			factura.TasaCambioBCN, factura.SubtotalGravado, factura.SubtotalExento, factura.MontoIVA,
			factura.Total, montoLetrasStr, factura.SaldoPendiente, factura.Estado, factura.Observaciones)
		if err != nil {
			return fmt.Errorf("error al guardar cabecera de factura: %w", err)
		}

		// Insertar Detalles
		for i := range detallesInsert {
			detallesInsert[i].FacturaID = facturaID
			_, err = tx.Exec(ctx,
				`INSERT INTO factura_detalles (id, factura_id, descripcion, cantidad, precio_unitario, exento, subtotal, monto_iva, total_linea)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
				detallesInsert[i].ID, detallesInsert[i].FacturaID, detallesInsert[i].Descripcion,
				detallesInsert[i].Cantidad, detallesInsert[i].PrecioUnitario, detallesInsert[i].Exento,
				detallesInsert[i].Subtotal, detallesInsert[i].MontoIVA, detallesInsert[i].TotalLinea)
			if err != nil {
				return fmt.Errorf("error al insertar detalle de factura: %w", err)
			}
		}

		// Insertar Cuotas si aplica
		for i := range cuotasInsert {
			cuotasInsert[i].FacturaID = facturaID
			_, err = tx.Exec(ctx,
				`INSERT INTO facturas_cuotas (id, factura_id, numero_cuota, fecha_vence, monto, saldo, estado)
				 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				cuotasInsert[i].ID, cuotasInsert[i].FacturaID, cuotasInsert[i].NumeroCuota,
				cuotasInsert[i].FechaVence, cuotasInsert[i].Monto, cuotasInsert[i].Saldo, cuotasInsert[i].Estado)
			if err != nil {
				return fmt.Errorf("error al insertar cuota de factura: %w", err)
			}
		}

		// 8. ASINTO CONTABLE AUTOMÁTICO DE EMISIÓN (Partida Doble Obligatoria)
		err = s.crearAsientoEmisionFactura(ctx, tx, factura)
		if err != nil {
			return fmt.Errorf("error al registrar asiento contable: %w", err)
		}

		// 9. Generar Payload ESC/P2 para Epson LQ-590
		escp2Bytes := domain.GenerarPayloadESCP2LQ590(factura, cliente, detallesInsert, cuotasInsert)

		resp.Factura = factura
		resp.Detalles = detallesInsert
		resp.Cuotas = cuotasInsert
		resp.Escp2Payload = base64.StdEncoding.EncodeToString(escp2Bytes)

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// crearAsientoEmisionFactura genera las líneas del libro diario asegurando DEBE == HABER en NIO y USD
func (s *FacturacionService) crearAsientoEmisionFactura(ctx context.Context, tx pgx.Tx, f domain.Factura) error {
	asientoID := uuid.New()
	numeroAsiento := fmt.Sprintf("ASI-FAC-%s", f.CorrelativoPreimpreso)
	concepto := fmt.Sprintf("Emisión de Factura Preimpresa %s - Cliente %s", f.CorrelativoPreimpreso, f.ClienteID)

	_, err := tx.Exec(ctx,
		`INSERT INTO asientos_contables (id, numero_asiento, fecha, concepto, origen, referencia_id)
		 VALUES ($1, $2, $3, $4, 'FACTURA_EMISION', $5)`,
		asientoID, numeroAsiento, f.FechaEmision, concepto, f.ID)
	if err != nil {
		return err
	}

	// Obtener IDs de cuentas contables del catálogo
	var cxcID, ingresoID, ivaID uuid.UUID
	_ = tx.QueryRow(ctx, `SELECT id FROM cuentas_contables WHERE codigo = '1103'`).Scan(&cxcID)
	_ = tx.QueryRow(ctx, `SELECT id FROM cuentas_contables WHERE codigo = '4101'`).Scan(&ingresoID)
	_ = tx.QueryRow(ctx, `SELECT id FROM cuentas_contables WHERE codigo = '2102'`).Scan(&ivaID)

	// Equivalencias en NIO y USD según la tasa oficial BCN
	totalNIO := f.Total
	totalUSD := f.Total / f.TasaCambioBCN
	if f.Moneda == "USD" {
		totalNIO = f.Total * f.TasaCambioBCN
		totalUSD = f.Total
	}

	subtotalNIO := (f.SubtotalGravado + f.SubtotalExento)
	subtotalUSD := subtotalNIO / f.TasaCambioBCN
	if f.Moneda == "USD" {
		subtotalNIO = (f.SubtotalGravado + f.SubtotalExento) * f.TasaCambioBCN
		subtotalUSD = f.SubtotalGravado + f.SubtotalExento
	}

	ivaNIO := f.MontoIVA
	ivaUSD := ivaNIO / f.TasaCambioBCN
	if f.Moneda == "USD" {
		ivaNIO = f.MontoIVA * f.TasaCambioBCN
		ivaUSD = f.MontoIVA
	}

	// 1. DEBE: Cuentas por Cobrar Clientes (1103)
	_, err = tx.Exec(ctx,
		`INSERT INTO asiento_lineas (id, asiento_id, cuenta_id, debe, haber, debe_usd, haber_usd)
		 VALUES ($1, $2, $3, $4, 0, $5, 0)`,
		uuid.New(), asientoID, cxcID, totalNIO, totalUSD)
	if err != nil {
		return err
	}

	// 2. HABER: Ingresos por Eventos Corporativos (4101)
	_, err = tx.Exec(ctx,
		`INSERT INTO asiento_lineas (id, asiento_id, cuenta_id, debe, haber, debe_usd, haber_usd)
		 VALUES ($1, $2, $3, 0, $4, 0, $5)`,
		uuid.New(), asientoID, ingresoID, subtotalNIO, subtotalUSD)
	if err != nil {
		return err
	}

	// 3. HABER: Débito Fiscal IVA 15% (2102) si aplica
	if ivaNIO > 0 {
		_, err = tx.Exec(ctx,
			`INSERT INTO asiento_lineas (id, asiento_id, cuenta_id, debe, haber, debe_usd, haber_usd)
			 VALUES ($1, $2, $3, 0, $4, 0, $5)`,
			uuid.New(), asientoID, ivaID, ivaNIO, ivaUSD)
		if err != nil {
			return err
		}
	}

	// Validación estricta de partida doble DEBE == HABER
	var sumaDebe, sumaHaber float64
	err = tx.QueryRow(ctx, `SELECT SUM(debe), SUM(haber) FROM asiento_lineas WHERE asiento_id = $1`, asientoID).Scan(&sumaDebe, &sumaHaber)
	if err != nil || math.Abs(sumaDebe-sumaHaber) > 0.01 {
		return fmt.Errorf("error en el asiento contable: Partida desbalanceada DEBE=%.2f HABER=%.2f", sumaDebe, sumaHaber)
	}

	return nil
}

// AnularFacturaPreimpresa anula una factura preimpresa (prohibido DELETE) y genera contrasiento contable
func (s *FacturacionService) AnularFacturaPreimpresa(ctx context.Context, facturaID uuid.UUID, motivo string) error {
	return s.Repo.ExecTx(ctx, func(tx pgx.Tx) error {
		var f domain.Factura
		err := tx.QueryRow(ctx,
			`SELECT id, correlativo_preimpreso, total, moneda, tasa_cambio_bcn, estado FROM facturas WHERE id = $1`, facturaID).
			Scan(&f.ID, &f.CorrelativoPreimpreso, &f.Total, &f.Moneda, &f.TasaCambioBCN, &f.Estado)
		if err != nil {
			return fmt.Errorf("factura no encontrada: %w", err)
		}

		if f.Estado == "ANULADA" {
			return fmt.Errorf("la factura %s ya se encuentra anulada", f.CorrelativoPreimpreso)
		}

		// Cambiar estado a ANULADA
		_, err = tx.Exec(ctx, `UPDATE facturas SET estado = 'ANULADA', observaciones = observaciones || ' | ANULADA: ' || $2 WHERE id = $1`, f.ID, motivo)
		if err != nil {
			return err
		}

		// Generar Contrasiento Contable Automático de Anulación
		asientoID := uuid.New()
		numeroAsiento := fmt.Sprintf("ASI-ANU-%s", f.CorrelativoPreimpreso)
		concepto := fmt.Sprintf("Anulación de Factura Preimpresa %s - Motivo: %s", f.CorrelativoPreimpreso, motivo)

		_, err = tx.Exec(ctx,
			`INSERT INTO asientos_contables (id, numero_asiento, fecha, concepto, origen, referencia_id)
			 VALUES ($1, $2, CURRENT_DATE, $3, 'FACTURA_ANULACION', $4)`,
			asientoID, numeroAsiento, concepto, f.ID)
		if err != nil {
			return err
		}

		// Invertir líneas del asiento original
		var originalID uuid.UUID
		_ = tx.QueryRow(ctx, `SELECT id FROM asientos_contables WHERE referencia_id = $1 AND origen = 'FACTURA_EMISION'`, f.ID).Scan(&originalID)

		rows, err := tx.Query(ctx, `SELECT cuenta_id, debe, haber, debe_usd, haber_usd FROM asiento_lineas WHERE asiento_id = $1`, originalID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var cID uuid.UUID
				var d, h, dUSD, hUSD float64
				_ = rows.Scan(&cID, &d, &h, &dUSD, &hUSD)
				// Inversión: Lo que era DEBE pasa a HABER y viceversa
				_, _ = tx.Exec(ctx,
					`INSERT INTO asiento_lineas (id, asiento_id, cuenta_id, debe, haber, debe_usd, haber_usd)
					 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
					uuid.New(), asientoID, cID, h, d, hUSD, dUSD)
			}
		}

		return nil
	})
}
