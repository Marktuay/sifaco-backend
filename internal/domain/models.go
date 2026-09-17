package domain

import (
	"time"

	"github.com/google/uuid"
)

// TalonarioDGI representa la autorización de folios preimpresos DGI
type TalonarioDGI struct {
	ID                 uuid.UUID `json:"id"`
	NumeroAutorizacion string    `json:"numero_autorizacion"`
	Serie              string    `json:"serie"`
	NumeroDesde        int       `json:"numero_desde"`
	NumeroHasta        int       `json:"numero_hasta"`
	NumeroActual       int       `json:"numero_actual"`
	FechaAutorizacion  time.Time `json:"fecha_autorizacion"`
	FechaVencimiento   time.Time `json:"fecha_vencimiento"`
	Activo             bool      `json:"activo"`
	CreadoEn           time.Time `json:"creado_en"`
}

// Cliente representa un cliente corporativo o persona natural
type Cliente struct {
	ID                 uuid.UUID `json:"id"`
	RuccEdula          string    `json:"ruc_cedula"`
	RazonSocial        string    `json:"razon_social"`
	Direccion          string    `json:"direccion"`
	Telefono           string    `json:"telefono"`
	Email              string    `json:"email"`
	RepresentanteLegal string    `json:"representante_legal"`
	TipoContribuyente  string    `json:"tipo_contribuyente"` // GRAN_CONTRIBUYENTE, REGIMEN_GENERAL, CUOTA_FIJA
	CreadoEn           time.Time `json:"creado_en"`
}

type CrearClienteRequest struct {
	RuccEdula          string `json:"ruc_cedula" binding:"required"`
	RazonSocial        string `json:"razon_social" binding:"required"`
	Direccion          string `json:"direccion"`
	Telefono           string `json:"telefono"`
	Email              string `json:"email"`
	RepresentanteLegal string `json:"representante_legal"`
	TipoContribuyente  string `json:"tipo_contribuyente"`
}

// Proveedor representa un suplidor de bienes o servicios
type Proveedor struct {
	ID           uuid.UUID `json:"id"`
	RuccEdula    string    `json:"ruc_cedula"`
	RazonSocial  string    `json:"razon_social"`
	Direccion    string    `json:"direccion"`
	Telefono     string    `json:"telefono"`
	Email        string    `json:"email"`
	RetenerIR    bool      `json:"retener_ir"`
	TipoServicio string    `json:"tipo_servicio"` // SERVICIOS_GENERALES, SERVICIOS_PROFESIONALES
	CreadoEn     time.Time `json:"creado_en"`
}

// Evento representa un taller, charla o congreso corporativo
type Evento struct {
	ID                   uuid.UUID `json:"id"`
	Codigo               string    `json:"codigo"`
	Nombre               string    `json:"nombre"`
	Descripcion          string    `json:"descripcion"`
	FechaInicio          time.Time `json:"fecha_inicio"`
	FechaFin             time.Time `json:"fecha_fin"`
	Lugar                string    `json:"lugar"`
	Capacidad            int       `json:"capacidad"`
	CantidadStands       int       `json:"cantidad_stands"`
	CantidadSalones      int       `json:"cantidad_salones"`
	CateringRequerido    bool      `json:"catering_requerido"`
	AudiovisualRequerido bool      `json:"audiovisual_requerido"`
	Estado               string    `json:"estado"`
	CreadoEn             time.Time `json:"creado_en"`
}

// Factura representa la cabecera de la factura preimpresa
type Factura struct {
	ID                    uuid.UUID `json:"id"`
	TalonarioID           uuid.UUID `json:"talonario_id"`
	CorrelativoPreimpreso string    `json:"correlativo_preimpreso"`
	Serie                 string    `json:"serie"`
	NumeroFolio           int       `json:"numero_folio"`
	ClienteID             uuid.UUID `json:"cliente_id"`
	EventoID              *uuid.UUID `json:"evento_id,omitempty"`
	FechaEmision          time.Time `json:"fecha_emision"`
	TipoPago              string    `json:"tipo_pago"` // CONTADO, CREDITO
	Moneda                string    `json:"moneda"`    // NIO, USD
	TasaCambioBCN         float64   `json:"tasa_cambio_bcn"`
	SubtotalGravado       float64   `json:"subtotal_gravado"`
	SubtotalExento        float64   `json:"subtotal_exento"`
	MontoIVA              float64   `json:"monto_iva"`
	Total                 float64   `json:"total"`
	MontoLetras           string    `json:"monto_letras"`
	SaldoPendiente        float64   `json:"saldo_pendiente"`
	Estado                string    `json:"estado"` // EMITIDA, ANULADA
	Observaciones         string    `json:"observaciones"`
	CreadoEn              time.Time `json:"creado_en"`
}

// FacturaDetalle representa la línea de detalle de la factura
type FacturaDetalle struct {
	ID             uuid.UUID `json:"id"`
	FacturaID      uuid.UUID `json:"factura_id"`
	Descripcion    string    `json:"descripcion"`
	Cantidad       float64   `json:"cantidad"`
	PrecioUnitario float64   `json:"precio_unitario"`
	Exento         bool      `json:"exento"`
	Subtotal       float64   `json:"subtotal"`
	MontoIVA       float64   `json:"monto_iva"`
	TotalLinea     float64   `json:"total_linea"`
}

// FacturaCuota representa una cuota programada para ventas al crédito
type FacturaCuota struct {
	ID          uuid.UUID `json:"id"`
	FacturaID   uuid.UUID `json:"factura_id"`
	NumeroCuota int       `json:"numero_cuota"`
	FechaVence  time.Time `json:"fecha_vence"`
	Monto       float64   `json:"monto"`
	Saldo       float64   `json:"saldo"`
	Estado      string    `json:"estado"` // PENDIENTE, PAGADA, PARCIAL
}

// DTOs para Creación de Factura Preimpresa
type DetalleRequest struct {
	Descripcion    string  `json:"descripcion" binding:"required"`
	Cantidad       float64 `json:"cantidad" binding:"required,gt=0"`
	PrecioUnitario float64 `json:"precio_unitario" binding:"required,gte=0"`
	Exento         bool    `json:"exento"`
}

type CuotaRequest struct {
	NumeroCuota int       `json:"numero_cuota" binding:"required"`
	FechaVence  time.Time `json:"fecha_vence" binding:"required"`
	Monto       float64   `json:"monto" binding:"required,gt=0"`
}

type CrearFacturaRequest struct {
	ClienteID     uuid.UUID        `json:"cliente_id" binding:"required"`
	EventoID      *uuid.UUID       `json:"evento_id"`
	TipoPago      string           `json:"tipo_pago" binding:"required"` // CONTADO, CREDITO
	Moneda        string           `json:"moneda" binding:"required"`    // NIO, USD
	TasaCambioBCN float64          `json:"tasa_cambio_bcn" binding:"required"`
	Observaciones string           `json:"observaciones"`
	Detalles      []DetalleRequest `json:"detalles" binding:"required,min=1"`
	Cuotas        []CuotaRequest   `json:"cuotas"`
}

type CrearFacturaResponse struct {
	Factura       Factura          `json:"factura"`
	Detalles      []FacturaDetalle `json:"detalles"`
	Cuotas        []FacturaCuota   `json:"cuotas"`
	Escp2Payload  string           `json:"escp2_payload_base64"` // Payload binario ESC/P2 codificado en base64 para impresión
}

// DTOs para Registro de Pagos / Recaudos de Cartera (CxC)
type RetencionInput struct {
	TipoRetencion     string  `json:"tipo_retencion" binding:"required"` // IR_2, IR_1, ALMA_1
	NumeroComprobante string  `json:"numero_comprobante" binding:"required"`
	BaseImponible     float64 `json:"base_imponible" binding:"required"`
	Porcentaje        float64 `json:"porcentaje" binding:"required"`
	MontoRetenido     float64 `json:"monto_retenido" binding:"required"`
}

type RegistrarPagoCuotaRequest struct {
	ClienteID     uuid.UUID        `json:"cliente_id" binding:"required"`
	CuotaID       uuid.UUID        `json:"cuota_id" binding:"required"`
	MontoPago     float64          `json:"monto_pago" binding:"required,gt=0"`
	Moneda        string           `json:"moneda" binding:"required"`
	TasaCambio    float64          `json:"tasa_cambio" binding:"required"`
	Notas         string           `json:"notas"`
	Retenciones   []RetencionInput `json:"retenciones"`
}

type FacturaPagoItem struct {
	FacturaID         uuid.UUID `json:"factura_id"`
	CuotaID           uuid.UUID `json:"cuota_id"`
	FechaFactura      string    `json:"fecha_factura"`
	NumeroFactura     string    `json:"numero_factura" binding:"required"`
	Subtotal          float64   `json:"subtotal"`
	NumeroNotaCredito string    `json:"numero_nota_credito"`
	MontoNotaCredito  float64   `json:"monto_nota_credito"`
	PorcentajeDesc    float64   `json:"porcentaje_desc"`
	MontoDescuento    float64   `json:"monto_descuento"`
	TotalPagado       float64   `json:"total_pagado" binding:"required,gt=0"`
}

type RegistrarReciboMultiFacturaRequest struct {
	NumeroRecibo  string            `json:"numero_recibo" binding:"required"` // ej. 5410
	ClienteID     uuid.UUID         `json:"cliente_id" binding:"required"`
	CodigoCliente string            `json:"codigo_cliente"`
	RecibimosDe   string            `json:"recibimos_de" binding:"required"` // ej. Farmacia DISPOER - Estelí
	Fecha         time.Time         `json:"fecha" binding:"required"`
	Concepto      string            `json:"concepto" binding:"required"`    // ej. Cancelacion fact. No. 3908 y 3992
	Banco         string            `json:"banco" binding:"required"`       // ej. LAFISE
	NumeroRef     string            `json:"numero_referencia" binding:"required"` // ej. TF No. 147299145
	Moneda        string            `json:"moneda" binding:"required"`      // NIO, USD
	TasaCambio    float64           `json:"tasa_cambio" binding:"required"`
	Facturas      []FacturaPagoItem `json:"facturas" binding:"required,min=1"`
	Retenciones   []RetencionInput  `json:"retenciones"`
}

type ReciboCaja struct {
	ID            uuid.UUID `json:"id"`
	NumeroRecibo  string    `json:"numero_recibo"`
	ClienteID     uuid.UUID `json:"cliente_id"`
	Fecha         time.Time `json:"fecha"`
	Moneda        string    `json:"moneda"`
	TasaCambio    float64   `json:"tasa_cambio"`
	TotalRecibido float64   `json:"total_recibido"`
	Estado        string    `json:"estado"`
	Notas         string    `json:"notas"`
}

// DTOs para Reportes Fiscales DGI
type FacturaDGIReporteItem struct {
	CorrelativoPreimpreso string  `json:"correlativo_preimpreso"`
	FechaEmision          string  `json:"fecha_emision"`
	RuccEdula             string  `json:"ruc_cedula"`
	RazonSocial           string  `json:"razon_social"`
	SubtotalGravadoNIO    float64 `json:"subtotal_gravado_nio"`
	SubtotalExentoNIO     float64 `json:"subtotal_exento_nio"`
	IVATrasladadoNIO      float64 `json:"iva_trasladado_nio"`
	TotalNIO              float64 `json:"total_nio"`
	Estado                string  `json:"estado"`
	MotivoAnulacion       string  `json:"motivo_anulacion,omitempty"`
}

type InformeVentasDGIResponse struct {
	Mes                   int                     `json:"mes"`
	Anio                  int                     `json:"anio"`
	RangoFoliosUtilizados string                  `json:"rango_folios_utilizados"`
	TotalFacturasEmitidas int                     `json:"total_facturas_emitidas"`
	TotalFacturasAnuladas int                     `json:"total_facturas_anuladas"`
	Facturas              []FacturaDGIReporteItem `json:"facturas"`
	TotalSubtotalGravado  float64                 `json:"total_subtotal_gravado_nio"`
	TotalSubtotalExento   float64                 `json:"total_subtotal_exento_nio"`
	TotalIVADebito        float64                 `json:"total_iva_debito_nio"`
	TotalGeneralVentas    float64                 `json:"total_general_ventas_nio"`
}

type RetencionRecibidaReporteItem struct {
	Fecha             string  `json:"fecha"`
	NumeroComprobante string  `json:"numero_comprobante"`
	RetenedorRUC      string  `json:"retenedor_ruc"`
	RetenedorNombre   string  `json:"retenedor_nombre"`
	FacturaReferencia string  `json:"factura_referencia"`
	TipoRetencion     string  `json:"tipo_retencion"`
	BaseImponibleNIO  float64 `json:"base_imponible_nio"`
	Porcentaje        float64 `json:"porcentaje"`
	MontoRetenidoNIO  float64 `json:"monto_retenido_nio"`
}

type InformeRetencionesRecibidasResponse struct {
	Mes              int                            `json:"mes"`
	Anio             int                            `json:"anio"`
	Retenciones      []RetencionRecibidaReporteItem `json:"retenciones"`
	TotalRetenidoIR  float64                        `json:"total_retenido_ir_nio"`
	TotalRetenidoALMA float64                       `json:"total_retenido_alma_nio"`
}

type RetencionEfectuadaReporteItem struct {
	Fecha             string  `json:"fecha"`
	NumeroComprobante string  `json:"numero_comprobante"`
	ProveedorRUC      string  `json:"proveedor_ruc"`
	ProveedorNombre   string  `json:"proveedor_nombre"`
	FacturaProveedor  string  `json:"factura_proveedor"`
	TipoRetencion     string  `json:"tipo_retencion"`
	BaseImponibleNIO  float64 `json:"base_imponible_nio"`
	Porcentaje        float64 `json:"porcentaje"`
	MontoRetenidoNIO  float64 `json:"monto_retenido_nio"`
}

type InformeRetencionesEfectuadasResponse struct {
	Mes                   int                             `json:"mes"`
	Anio                  int                             `json:"anio"`
	Retenciones           []RetencionEfectuadaReporteItem `json:"retenciones"`
	TotalRetenidoServicios float64                        `json:"total_retenido_servicios_2_nio"`
	TotalRetenidoProf     float64                         `json:"total_retenido_profesionales_10_nio"`
}

type DashboardKPIsResponse struct {
	VentasMesNIO        float64            `json:"ventas_mes_nio"`
	VentasMesUSD        float64            `json:"ventas_mes_usd"`
	CarteraCxCVencida   float64            `json:"cartera_cxc_vencida_nio"`
	CarteraCxCPorVencer float64            `json:"cartera_cxc_por_vencer_nio"`
	IVANetoEstimado     float64            `json:"iva_neto_estimado_nio"`
	AntiguedadSaldos    map[string]float64 `json:"antiguedad_saldos_cxc"` // "0-30", "31-60", "61-90", "90+"
}

type AuditoriaLogItem struct {
	ID            uuid.UUID `json:"id"`
	UsuarioNombre string    `json:"usuario_nombre"`
	UsuarioEmail  string    `json:"usuario_email"`
	Rol           string    `json:"rol"`
	Accion        string    `json:"accion"`
	TablaAfectada string    `json:"tabla_afectada"`
	Detalles      string    `json:"detalles"`
	IPOrigen      string    `json:"ip_origen"`
	FechaHora     string    `json:"fecha_hora"`
	CreadoEn      time.Time `json:"creado_en"`
}

type RegistrarAuditoriaRequest struct {
	UsuarioEmail  string `json:"usuario_email"`
	Rol           string `json:"rol"`
	Accion        string `json:"accion"`
	TablaAfectada string `json:"tabla_afectada"`
	Detalles      string `json:"detalles"`
}

type BackupItem struct {
	ID             uuid.UUID `json:"id"`
	NombreArchivo  string    `json:"nombre_archivo"`
	RutaAbsoluta   string    `json:"ruta_absoluta"`
	TamanoMB       float64   `json:"tamano_mb"`
	ChecksumSHA256 string    `json:"checksum_sha256"`
	TipoBackup     string    `json:"tipo_backup"` // COMPLETO, INCREMENTAL, MANUAL
	Estado         string    `json:"estado"`      // VALIDO, RESTAURADO, CORRUPTO
	FechaHora      string    `json:"fecha_hora"`
	CreadoEn       time.Time `json:"creado_en"`
}

type DRPStatusResponse struct {
	EstadoSalud        string       `json:"estado_salud"` // PROTEGIDO, ATENCION, CRITICO
	UltimoBackupFecha  string       `json:"ultimo_backup_fecha"`
	UltimoBackupNombre string       `json:"ultimo_backup_nombre"`
	TotalBackups       int          `json:"total_backups"`
	EspacioUtilizadoMB float64      `json:"espacio_utilizado_mb"`
	RPOStatus          string       `json:"rpo_status"` // Cumplido (< 15 min)
	RTOStatus          string       `json:"rto_status"` // Listo (< 30 min)
	BackupsRecientes   []BackupItem `json:"backups_recientes"`
}

type RestaurarBackupRequest struct {
	NombreArchivo string `json:"nombre_archivo" binding:"required"`
}

// Structs para Gestión de Usuarios y Roles (RBAC)
type Usuario struct {
	ID           uuid.UUID `json:"id"`
	Nombre       string    `json:"nombre"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // Oculto en respuestas JSON
	Rol          string    `json:"rol"` // ADMIN, CONTADOR, FACTURADOR, GESTOR_CXC, COORDINADOR_EVENTOS
	Activo       bool      `json:"activo"`
	CreadoEn     time.Time `json:"creado_en"`
}

type CrearUsuarioRequest struct {
	Nombre   string `json:"nombre" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	Rol      string `json:"rol" binding:"required"`
}

type ActualizarUsuarioRequest struct {
	Nombre   string `json:"nombre"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Rol      string `json:"rol"`
	Activo   *bool  `json:"activo"`
}

