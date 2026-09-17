package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"sifaco/backend/internal/domain"
	"sifaco/backend/internal/repository/postgres"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type CarteraService struct {
	Repo *postgres.Repository
}

func NewCarteraService(repo *postgres.Repository) *CarteraService {
	return &CarteraService{Repo: repo}
}

// RegistrarPagoCuota procesa el abono/liquidación de una cuota de crédito con comprobantes de retención DGI/ALMA
func (s *CarteraService) RegistrarPagoCuota(ctx context.Context, req domain.RegistrarPagoCuotaRequest) (string, error) {
	if s == nil || s.Repo == nil || s.Repo.Pool == nil {
		return "", fmt.Errorf("base de datos no disponible")
	}

	var numeroRecibo string

	err := s.Repo.ExecTx(ctx, func(tx pgx.Tx) error {
		// 1. Obtener y Bloquear Cuota
		var cuota domain.FacturaCuota
		var facturaID uuid.UUID
		var facturaCorrelativo string

		err := tx.QueryRow(ctx,
			`SELECT c.id, c.factura_id, c.numero_cuota, c.monto, c.saldo, c.estado, f.correlativo_preimpreso
			 FROM facturas_cuotas c
			 JOIN facturas f ON f.id = c.factura_id
			 WHERE c.id = $1
			 FOR UPDATE`, req.CuotaID).
			Scan(&cuota.ID, &facturaID, &cuota.NumeroCuota, &cuota.Monto, &cuota.Saldo, &cuota.Estado, &facturaCorrelativo)
		if err != nil {
			return fmt.Errorf("cuota no encontrada: %w", err)
		}

		if cuota.Saldo <= 0 || cuota.Estado == "PAGADA" {
			return fmt.Errorf("la cuota #%d de la factura %s ya se encuentra totalmente cancelada", cuota.NumeroCuota, facturaCorrelativo)
		}

		if req.MontoPago > cuota.Saldo {
			return fmt.Errorf("el monto a pagar (%.2f) excede el saldo pendiente de la cuota (%.2f)", req.MontoPago, cuota.Saldo)
		}

		// 2. Calcular Retenciones Practicadas por el Cliente (DGI 2%/1% y ALMA 1%)
		var totalRetencionIR, totalRetencionALMA float64
		for _, r := range req.Retenciones {
			switch r.TipoRetencion {
			case "IR_2", "IR_1":
				totalRetencionIR += r.MontoRetenido
			case "ALMA_1":
				totalRetencionALMA += r.MontoRetenido
			default:
				return fmt.Errorf("tipo de retención no válido: %s", r.TipoRetencion)
			}
		}

		totalRetenciones := totalRetencionIR + totalRetencionALMA
		netoBancoCaja := req.MontoPago - totalRetenciones

		if netoBancoCaja < 0 {
			return fmt.Errorf("el total de retenciones (%.2f) supera el monto bruto abonado (%.2f)", totalRetenciones, req.MontoPago)
		}

		// 3. Crear Recibo de Caja
		reciboID := uuid.New()
		numeroRecibo = fmt.Sprintf("RC-%s-C%d-%d", facturaCorrelativo, cuota.NumeroCuota, time.Now().Unix()%10000)

		_, err = tx.Exec(ctx,
			`INSERT INTO recibos_caja (id, numero_recibo, cliente_id, fecha, moneda, tasa_cambio, total_recibido, notas)
			 VALUES ($1, $2, $3, CURRENT_DATE, $4, $5, $6, $7)`,
			reciboID, numeroRecibo, req.ClienteID, req.Moneda, req.TasaCambio, req.MontoPago, req.Notas)
		if err != nil {
			return fmt.Errorf("error al crear recibo de caja: %w", err)
		}

		// Detalle de Recibo
		_, err = tx.Exec(ctx,
			`INSERT INTO recibo_caja_detalles (id, recibo_id, cuota_id, monto_aplicado)
			 VALUES ($1, $2, $3, $4)`,
			uuid.New(), reciboID, cuota.ID, req.MontoPago)
		if err != nil {
			return err
		}

		// 4. Registrar Comprobantes Físicos de Retenciones Recibidas
		for _, ret := range req.Retenciones {
			_, err = tx.Exec(ctx,
				`INSERT INTO retenciones_recibidas (id, recibo_id, factura_id, tipo_retencion, numero_comprobante, base_imponible, porcentaje, monto_retenido)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
				uuid.New(), reciboID, facturaID, ret.TipoRetencion, ret.NumeroComprobante, ret.BaseImponible, ret.Porcentaje, ret.MontoRetenido)
			if err != nil {
				return fmt.Errorf("error al guardar comprobante de retención %s: %w", ret.NumeroComprobante, err)
			}
		}

		// 5. Actualizar Saldo de Cuota y Factura
		nuevoSaldoCuota := cuota.Saldo - req.MontoPago
		nuevoEstadoCuota := "PARCIAL"
		if nuevoSaldoCuota <= 0.001 {
			nuevoSaldoCuota = 0
			nuevoEstadoCuota = "PAGADA"
		}

		_, err = tx.Exec(ctx, `UPDATE facturas_cuotas SET saldo = $1, estado = $2 WHERE id = $3`, nuevoSaldoCuota, nuevoEstadoCuota, cuota.ID)
		if err != nil {
			return err
		}

		_, err = tx.Exec(ctx, `UPDATE facturas SET saldo_pendiente = GREATEST(0, saldo_pendiente - $1) WHERE id = $2`, req.MontoPago, facturaID)
		if err != nil {
			return err
		}

		// 6. ASIENTO CONTABLE AUTOMÁTICO DE RECAUDO
		asientoID := uuid.New()
		numeroAsiento := fmt.Sprintf("ASI-REC-%s", numeroRecibo)
		concepto := fmt.Sprintf("Recaudo Cuota #%d Factura %s - Recibo %s", cuota.NumeroCuota, facturaCorrelativo, numeroRecibo)

		_, err = tx.Exec(ctx,
			`INSERT INTO asientos_contables (id, numero_asiento, fecha, concepto, origen, referencia_id)
			 VALUES ($1, $2, CURRENT_DATE, $3, 'RECAUDO_CUOTA', $4)`,
			asientoID, numeroAsiento, concepto, reciboID)
		if err != nil {
			return err
		}

		// Obtener cuentas contables
		var bancoID, irRetenidoID, almaRetenidoID, cxcID uuid.UUID
		_ = tx.QueryRow(ctx, `SELECT id FROM cuentas_contables WHERE codigo = '1102'`).Scan(&bancoID)
		_ = tx.QueryRow(ctx, `SELECT id FROM cuentas_contables WHERE codigo = '1104'`).Scan(&irRetenidoID)
		_ = tx.QueryRow(ctx, `SELECT id FROM cuentas_contables WHERE codigo = '1105'`).Scan(&almaRetenidoID)
		_ = tx.QueryRow(ctx, `SELECT id FROM cuentas_contables WHERE codigo = '1103'`).Scan(&cxcID)

		netoNIO := netoBancoCaja
		if req.Moneda == "USD" {
			netoNIO = netoBancoCaja * req.TasaCambio
		}
		montoBrutoNIO := req.MontoPago
		if req.Moneda == "USD" {
			montoBrutoNIO = req.MontoPago * req.TasaCambio
		}

		// 1. DEBE: Banco/Caja (Neto recibido después de retenciones)
		if netoNIO > 0 {
			_, err = tx.Exec(ctx, `INSERT INTO asiento_lineas (id, asiento_id, cuenta_id, debe, haber, debe_usd, haber_usd) VALUES ($1, $2, $3, $4, 0, $5, 0)`,
				uuid.New(), asientoID, bancoID, netoNIO, netoNIO/req.TasaCambio)
			if err != nil {
				return err
			}
		}

		// 2. DEBE: Anticipo IR Retenido 2%/1% (Si aplica)
		if totalRetencionIR > 0 {
			_, err = tx.Exec(ctx, `INSERT INTO asiento_lineas (id, asiento_id, cuenta_id, debe, haber, debe_usd, haber_usd) VALUES ($1, $2, $3, $4, 0, $5, 0)`,
				uuid.New(), asientoID, irRetenidoID, totalRetencionIR, totalRetencionIR/req.TasaCambio)
			if err != nil {
				return err
			}
		}

		// 3. DEBE: Anticipo ALMA Retenido 1% (Si aplica)
		if totalRetencionALMA > 0 {
			_, err = tx.Exec(ctx, `INSERT INTO asiento_lineas (id, asiento_id, cuenta_id, debe, haber, debe_usd, haber_usd) VALUES ($1, $2, $3, $4, 0, $5, 0)`,
				uuid.New(), asientoID, almaRetenidoID, totalRetencionALMA, totalRetencionALMA/req.TasaCambio)
			if err != nil {
				return err
			}
		}

		// 4. HABER: CxC Clientes (Monto bruto amortizado de la cuota)
		_, err = tx.Exec(ctx, `INSERT INTO asiento_lineas (id, asiento_id, cuenta_id, debe, haber, debe_usd, haber_usd) VALUES ($1, $2, $3, 0, $4, 0, $5)`,
			uuid.New(), asientoID, cxcID, montoBrutoNIO, montoBrutoNIO/req.TasaCambio)
		if err != nil {
			return err
		}

		// Validar Partida Doble
		var sDebe, sHaber float64
		_ = tx.QueryRow(ctx, `SELECT SUM(debe), SUM(haber) FROM asiento_lineas WHERE asiento_id = $1`, asientoID).Scan(&sDebe, &sHaber)
		if math.Abs(sDebe-sHaber) > 0.01 {
			return fmt.Errorf("error en asiento de recaudo: DEBE=%.2f no coincide con HABER=%.2f", sDebe, sHaber)
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	return numeroRecibo, nil
}

// RegistrarReciboMultiFactura procesa un Recibo Oficial de Caja estilo RINSA (N° 5410) aplicando pagos a múltiples facturas/cuotas de un cliente
func (s *CarteraService) RegistrarReciboMultiFactura(ctx context.Context, req domain.RegistrarReciboMultiFacturaRequest) (string, error) {
	if s == nil || s.Repo == nil || s.Repo.Pool == nil {
		return "", fmt.Errorf("base de datos no disponible")
	}

	if len(req.Facturas) == 0 {
		return "", fmt.Errorf("debe incluir al menos una factura a cancelar en el recibo")
	}

	var totalCobradoBruto float64
	for _, item := range req.Facturas {
		if item.TotalPagado <= 0 {
			return "", fmt.Errorf("el total pagado para la factura %s debe ser mayor a cero", item.NumeroFactura)
		}
		totalCobradoBruto += item.TotalPagado
	}

	err := s.Repo.ExecTx(ctx, func(tx pgx.Tx) error {
		// 1. Calcular Retenciones Físicas
		var totalRetencionIR, totalRetencionALMA float64
		for _, r := range req.Retenciones {
			switch r.TipoRetencion {
			case "IR_2", "IR_1":
				totalRetencionIR += r.MontoRetenido
			case "ALMA_1":
				totalRetencionALMA += r.MontoRetenido
			default:
				return fmt.Errorf("tipo de retención no válido: %s", r.TipoRetencion)
			}
		}

		totalRetenciones := totalRetencionIR + totalRetencionALMA
		netoBanco := totalCobradoBruto - totalRetenciones
		if netoBanco < 0 {
			return fmt.Errorf("el total de retenciones (%.2f) supera el total cobrado (%.2f)", totalRetenciones, totalCobradoBruto)
		}

		// 2. Insertar Recibo de Caja General
		reciboID := uuid.New()
		notas := fmt.Sprintf("%s | Banco: %s | Ref: %s | Recibimos de: %s", req.Concepto, req.Banco, req.NumeroRef, req.RecibimosDe)

		_, err := tx.Exec(ctx,
			`INSERT INTO recibos_caja (id, numero_recibo, cliente_id, fecha, moneda, tasa_cambio, total_recibido, notas)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			reciboID, req.NumeroRecibo, req.ClienteID, req.Fecha, req.Moneda, req.TasaCambio, totalCobradoBruto, notas)
		if err != nil {
			return fmt.Errorf("error al registrar recibo de caja #%s: %w", req.NumeroRecibo, err)
		}

		// 3. Procesar cada Factura / Cuota
		for _, item := range req.Facturas {
			// Bloquear cuota si fue enviada
			if item.CuotaID != uuid.Nil {
				var cuotaSaldo float64
				err := tx.QueryRow(ctx, `SELECT saldo FROM facturas_cuotas WHERE id = $1 FOR UPDATE`, item.CuotaID).Scan(&cuotaSaldo)
				if err == nil {
					nuevoSaldo := math.Max(0, cuotaSaldo-item.TotalPagado)
					nuevoEstado := "PARCIAL"
					if nuevoSaldo <= 0.001 {
						nuevoSaldo = 0
						nuevoEstado = "PAGADA"
					}
					_, _ = tx.Exec(ctx, `UPDATE facturas_cuotas SET saldo = $1, estado = $2 WHERE id = $3`, nuevoSaldo, nuevoEstado, item.CuotaID)
				}

				// Insertar detalle del recibo
				_, err = tx.Exec(ctx,
					`INSERT INTO recibo_caja_detalles (id, recibo_id, cuota_id, monto_aplicado)
					 VALUES ($1, $2, $3, $4)`,
					uuid.New(), reciboID, item.CuotaID, item.TotalPagado)
				if err != nil {
					return fmt.Errorf("error al insertar detalle de cuota %s: %w", item.CuotaID, err)
				}
			}

			// Actualizar saldo de factura principal
			if item.FacturaID != uuid.Nil {
				_, err = tx.Exec(ctx,
					`UPDATE facturas SET saldo_pendiente = GREATEST(0, saldo_pendiente - $1) WHERE id = $2`,
					item.TotalPagado, item.FacturaID)
				if err != nil {
					return fmt.Errorf("error al actualizar saldo de factura %s: %w", item.NumeroFactura, err)
				}
			}
		}

		// 4. Registrar Comprobantes de Retención Físicos Recibidos
		for _, ret := range req.Retenciones {
			facturaID := uuid.Nil
			if len(req.Facturas) > 0 {
				facturaID = req.Facturas[0].FacturaID
			}

			_, err = tx.Exec(ctx,
				`INSERT INTO retenciones_recibidas (id, recibo_id, factura_id, tipo_retencion, numero_comprobante, base_imponible, porcentaje, monto_retenido)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
				uuid.New(), reciboID, facturaID, ret.TipoRetencion, ret.NumeroComprobante, ret.BaseImponible, ret.Porcentaje, ret.MontoRetenido)
			if err != nil {
				return fmt.Errorf("error al guardar comprobante de retención %s: %w", ret.NumeroComprobante, err)
			}
		}

		// 5. ASIENTO CONTABLE AUTOMÁTICO MULTI-FACTURA
		asientoID := uuid.New()
		numeroAsiento := fmt.Sprintf("ASI-REC-%s", req.NumeroRecibo)
		concepto := fmt.Sprintf("Recaudo Multi-Factura Recibo %s (%s)", req.NumeroRecibo, req.Concepto)

		_, err = tx.Exec(ctx,
			`INSERT INTO asientos_contables (id, numero_asiento, fecha, concepto, origen, referencia_id)
			 VALUES ($1, $2, $3, $4, 'RECAUDO_MULTI_FACTURA', $5)`,
			asientoID, numeroAsiento, req.Fecha, concepto, reciboID)
		if err != nil {
			return fmt.Errorf("error al crear asiento contable: %w", err)
		}

		// Cuentas contables
		var bancoID, irRetenidoID, almaRetenidoID, cxcID uuid.UUID
		_ = tx.QueryRow(ctx, `SELECT id FROM cuentas_contables WHERE codigo = '1102'`).Scan(&bancoID)
		_ = tx.QueryRow(ctx, `SELECT id FROM cuentas_contables WHERE codigo = '1104'`).Scan(&irRetenidoID)
		_ = tx.QueryRow(ctx, `SELECT id FROM cuentas_contables WHERE codigo = '1105'`).Scan(&almaRetenidoID)
		_ = tx.QueryRow(ctx, `SELECT id FROM cuentas_contables WHERE codigo = '1103'`).Scan(&cxcID)

		tasa := req.TasaCambio
		if tasa <= 0 {
			tasa = 1.0
		}

		netoNIO := netoBanco
		montoBrutoNIO := totalCobradoBruto
		if req.Moneda == "USD" {
			netoNIO = netoBanco * tasa
			montoBrutoNIO = totalCobradoBruto * tasa
		}

		// 1. DEBE: Banco/Caja (Neto recibido)
		if netoNIO > 0 {
			_, err = tx.Exec(ctx, `INSERT INTO asiento_lineas (id, asiento_id, cuenta_id, debe, haber, debe_usd, haber_usd) VALUES ($1, $2, $3, $4, 0, $5, 0)`,
				uuid.New(), asientoID, bancoID, netoNIO, netoNIO/tasa)
			if err != nil {
				return err
			}
		}

		// 2. DEBE: Anticipo IR Retenido
		if totalRetencionIR > 0 {
			montoIRNIO := totalRetencionIR
			if req.Moneda == "USD" {
				montoIRNIO = totalRetencionIR * tasa
			}
			_, err = tx.Exec(ctx, `INSERT INTO asiento_lineas (id, asiento_id, cuenta_id, debe, haber, debe_usd, haber_usd) VALUES ($1, $2, $3, $4, 0, $5, 0)`,
				uuid.New(), asientoID, irRetenidoID, montoIRNIO, montoIRNIO/tasa)
			if err != nil {
				return err
			}
		}

		// 3. DEBE: Anticipo ALMA Retenido
		if totalRetencionALMA > 0 {
			montoALMANIO := totalRetencionALMA
			if req.Moneda == "USD" {
				montoALMANIO = totalRetencionALMA * tasa
			}
			_, err = tx.Exec(ctx, `INSERT INTO asiento_lineas (id, asiento_id, cuenta_id, debe, haber, debe_usd, haber_usd) VALUES ($1, $2, $3, $4, 0, $5, 0)`,
				uuid.New(), asientoID, almaRetenidoID, montoALMANIO, montoALMANIO/tasa)
			if err != nil {
				return err
			}
		}

		// 4. HABER: CxC Clientes (Monto bruto total amortizado)
		_, err = tx.Exec(ctx, `INSERT INTO asiento_lineas (id, asiento_id, cuenta_id, debe, haber, debe_usd, haber_usd) VALUES ($1, $2, $3, 0, $4, 0, $5)`,
			uuid.New(), asientoID, cxcID, montoBrutoNIO, montoBrutoNIO/tasa)
		if err != nil {
			return err
		}

		// Validar Partida Doble
		var sDebe, sHaber float64
		_ = tx.QueryRow(ctx, `SELECT SUM(debe), SUM(haber) FROM asiento_lineas WHERE asiento_id = $1`, asientoID).Scan(&sDebe, &sHaber)
		if math.Abs(sDebe-sHaber) > 0.01 {
			return fmt.Errorf("error en asiento de recaudo multi-factura: DEBE=%.2f no coincide con HABER=%.2f", sDebe, sHaber)
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	return req.NumeroRecibo, nil
}

// AnularReciboCaja anula un recibo oficial de caja (individual o multi-factura), restaura los saldos pendientes de cuotas/facturas y genera el contrasiento contable
func (s *CarteraService) AnularReciboCaja(ctx context.Context, reciboID uuid.UUID, motivo string) error {
	if s == nil || s.Repo == nil || s.Repo.Pool == nil {
		return fmt.Errorf("base de datos no disponible")
	}

	return s.Repo.ExecTx(ctx, func(tx pgx.Tx) error {
		// 1. Obtener y bloquear Recibo
		var r domain.ReciboCaja
		var estado string
		err := tx.QueryRow(ctx,
			`SELECT id, numero_recibo, cliente_id, total_recibido, notas, COALESCE(estado, 'EMITIDO')
			 FROM recibos_caja WHERE id = $1 FOR UPDATE`, reciboID).
			Scan(&r.ID, &r.NumeroRecibo, &r.ClienteID, &r.TotalRecibido, &r.Notas, &estado)
		if err != nil {
			return fmt.Errorf("recibo de caja no encontrado: %w", err)
		}

		if estado == "ANULADO" {
			return fmt.Errorf("el recibo de caja %s ya se encuentra anulado", r.NumeroRecibo)
		}

		// 2. Marcar Recibo como ANULADO
		_, err = tx.Exec(ctx, `UPDATE recibos_caja SET estado = 'ANULADO', notas = COALESCE(notas, '') || ' | ANULADO: ' || $2 WHERE id = $1`, r.ID, motivo)
		if err != nil {
			return fmt.Errorf("error al actualizar estado del recibo: %w", err)
		}

		// 3. Restaurar saldos de cuotas y facturas vinculadas
		rows, err := tx.Query(ctx, `SELECT cuota_id, monto_aplicado FROM recibo_caja_detalles WHERE recibo_id = $1`, r.ID)
		if err == nil {
			type detalleItem struct {
				cuotaID uuid.UUID
				monto   float64
			}
			var detalles []detalleItem
			for rows.Next() {
				var d detalleItem
				if err := rows.Scan(&d.cuotaID, &d.monto); err == nil {
					detalles = append(detalles, d)
				}
			}
			rows.Close()

			for _, item := range detalles {
				if item.cuotaID != uuid.Nil {
					var facturaID uuid.UUID
					var saldoActual, montoCuota float64
					err = tx.QueryRow(ctx, `SELECT factura_id, saldo, monto FROM facturas_cuotas WHERE id = $1 FOR UPDATE`, item.cuotaID).
						Scan(&facturaID, &saldoActual, &montoCuota)

					if err == nil {
						nuevoSaldo := math.Min(montoCuota, saldoActual+item.monto)
						nuevoEstado := "PARCIAL"
						if math.Abs(nuevoSaldo-montoCuota) < 0.001 {
							nuevoSaldo = montoCuota
							nuevoEstado = "PENDIENTE"
						}

						_, _ = tx.Exec(ctx, `UPDATE facturas_cuotas SET saldo = $1, estado = $2 WHERE id = $3`, nuevoSaldo, nuevoEstado, item.cuotaID)

						if facturaID != uuid.Nil {
							_, _ = tx.Exec(ctx, `UPDATE facturas SET saldo_pendiente = saldo_pendiente + $1 WHERE id = $2`, item.monto, facturaID)
						}
					}
				}
			}
		}

		// 4. Generar Contrasiento Contable Automático de Anulación de Recaudo
		asientoID := uuid.New()
		numeroAsiento := fmt.Sprintf("ASI-ANU-%s", r.NumeroRecibo)
		concepto := fmt.Sprintf("Anulación de Recibo Oficial de Caja %s - Motivo: %s", r.NumeroRecibo, motivo)

		_, err = tx.Exec(ctx,
			`INSERT INTO asientos_contables (id, numero_asiento, fecha, concepto, origen, referencia_id)
			 VALUES ($1, $2, CURRENT_DATE, $3, 'RECAUDO_ANULACION', $4)`,
			asientoID, numeroAsiento, concepto, r.ID)
		if err != nil {
			return fmt.Errorf("error al crear contrasiento contable: %w", err)
		}

		// Invertir las líneas del asiento de recaudo original
		var originalID uuid.UUID
		err = tx.QueryRow(ctx, `SELECT id FROM asientos_contables WHERE referencia_id = $1 AND (origen = 'RECAUDO_CUOTA' OR origen = 'RECAUDO_MULTI_FACTURA') ORDER BY creado_en DESC LIMIT 1`, r.ID).Scan(&originalID)
		if err == nil {
			lineRows, err := tx.Query(ctx, `SELECT cuenta_id, debe, haber, debe_usd, haber_usd FROM asiento_lineas WHERE asiento_id = $1`, originalID)
			if err == nil {
				defer lineRows.Close()
				for lineRows.Next() {
					var cID uuid.UUID
					var d, h, dUSD, hUSD float64
					_ = lineRows.Scan(&cID, &d, &h, &dUSD, &hUSD)
					// Inversión: DEBE -> HABER, HABER -> DEBE
					_, _ = tx.Exec(ctx,
						`INSERT INTO asiento_lineas (id, asiento_id, cuenta_id, debe, haber, debe_usd, haber_usd)
						 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
						uuid.New(), asientoID, cID, h, d, hUSD, dUSD)
				}
			}
		}

		return nil
	})
}


