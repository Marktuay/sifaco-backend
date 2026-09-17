-- ==============================================================================
-- SIFACO - Master DDL PostgreSQL 16 (8 Módulos Completos)
-- Ley 822 (LCT Nicaragua), DGI, ALMA, CxC, CxP, Contabilidad, Eventos & RBAC
-- ==============================================================================

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ------------------------------------------------------------------------------
-- MÓDULO 8: SEGURIDAD, RBAC Y PISTA DE AUDITORÍA (auditoria_logs)
-- ------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS usuarios (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    nombre VARCHAR(100) NOT NULL,
    email VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    rol VARCHAR(30) NOT NULL DEFAULT 'FACTURADOR', -- ADMIN, CONTADOR, FACTURADOR, OPERADOR_EVENTOS
    activo BOOLEAN NOT NULL DEFAULT TRUE,
    creado_en TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS auditoria_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    usuario_id UUID REFERENCES usuarios(id),
    accion VARCHAR(50) NOT NULL, -- CREAR_FACTURA, ANULAR_FACTURA, REGISTRAR_PAGO, CREAR_EVENTO
    tabla_afectada VARCHAR(50) NOT NULL,
    registro_id UUID,
    valores_anteriores JSONB,
    valores_nuevos JSONB,
    ip_origen VARCHAR(45),
    creado_en TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_auditoria_tabla ON auditoria_logs(tabla_afectada, registro_id);
CREATE INDEX idx_auditoria_fecha ON auditoria_logs(creado_en);

-- ------------------------------------------------------------------------------
-- MÓDULO 1: TALONARIOS DGI (Folios Preimpresos)
-- ------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS talonarios_dgi (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    numero_autorizacion VARCHAR(50) NOT NULL UNIQUE,
    serie VARCHAR(10) NOT NULL,
    numero_desde INT NOT NULL,
    numero_hasta INT NOT NULL,
    numero_actual INT NOT NULL,
    fecha_autorizacion DATE NOT NULL,
    fecha_vencimiento DATE NOT NULL,
    activo BOOLEAN NOT NULL DEFAULT TRUE,
    creado_en TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_rango_talonario CHECK (numero_hasta >= numero_desde AND numero_actual >= numero_desde AND numero_actual <= numero_hasta + 1)
);

CREATE INDEX idx_talonarios_dgi_activo ON talonarios_dgi(activo) WHERE activo IS TRUE;

-- ------------------------------------------------------------------------------
-- MÓDULO 3 & 4: CATÁLOGO DE CLIENTES Y PROVEEDORES
-- ------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS clientes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ruc_cedula VARCHAR(20) NOT NULL UNIQUE,
    razon_social VARCHAR(150) NOT NULL,
    direccion TEXT,
    telefono VARCHAR(20),
    email VARCHAR(100),
    representante_legal VARCHAR(150),
    tipo_contribuyente VARCHAR(30) NOT NULL DEFAULT 'GRAN_CONTRIBUYENTE', -- GRAN_CONTRIBUYENTE, REGIMEN_GENERAL, CUOTA_FIJA
    creado_en TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS proveedores (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ruc_cedula VARCHAR(20) NOT NULL UNIQUE,
    razon_social VARCHAR(150) NOT NULL,
    direccion TEXT,
    telefono VARCHAR(20),
    email VARCHAR(100),
    retener_ir BOOLEAN NOT NULL DEFAULT TRUE,
    tipo_servicio VARCHAR(30) NOT NULL DEFAULT 'SERVICIOS_GENERALES', -- SERVICIOS_GENERALES (2%), SERVICIOS_PROFESIONALES (10%)
    creado_en TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- ------------------------------------------------------------------------------
-- MÓDULO 5: CONTABILIDAD GENERAL (Catálogo de Cuentas)
-- ------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS cuentas_contables (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    codigo VARCHAR(20) NOT NULL UNIQUE,
    nombre VARCHAR(120) NOT NULL,
    tipo VARCHAR(20) NOT NULL, -- ACTIVO, PASIVO, PATRIMONIO, INGRESO, GASTO
    nivel INT NOT NULL DEFAULT 1,
    cuenta_padre_id UUID REFERENCES cuentas_contables(id),
    activa BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE INDEX idx_cuentas_codigo ON cuentas_contables(codigo);

-- ------------------------------------------------------------------------------
-- MÓDULO 6: GESTIÓN DE EVENTOS, TALLERES Y PAQUETES (Centros de Costo)
-- ------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS eventos (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    codigo VARCHAR(30) NOT NULL UNIQUE,
    nombre VARCHAR(150) NOT NULL,
    descripcion TEXT,
    fecha_inicio DATE NOT NULL,
    fecha_fin DATE NOT NULL,
    lugar VARCHAR(150),
    capacidad INT NOT NULL DEFAULT 100,
    cantidad_stands INT NOT NULL DEFAULT 0,
    cantidad_salones INT NOT NULL DEFAULT 1,
    catering_requerido BOOLEAN NOT NULL DEFAULT TRUE,
    audiovisual_requerido BOOLEAN NOT NULL DEFAULT TRUE,
    estado VARCHAR(20) NOT NULL DEFAULT 'ACTIVO',
    creado_en TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS evento_paquetes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    evento_id UUID NOT NULL REFERENCES eventos(id) ON DELETE CASCADE,
    nombre_paquete VARCHAR(100) NOT NULL, -- PATROCINIO_ORO, PATROCINIO_SILVER, ENTRADA_INDIVIDUAL, STAND_EXHIBICION
    precio NUMERIC(14,4) NOT NULL DEFAULT 0.0000,
    moneda VARCHAR(3) NOT NULL DEFAULT 'NIO'
);

-- ------------------------------------------------------------------------------
-- MÓDULO 1: FACTURAS DE VENTA (Cabecera y Detalle)
-- ------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS facturas (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    talonario_id UUID NOT NULL REFERENCES talonarios_dgi(id),
    correlativo_preimpreso VARCHAR(30) NOT NULL UNIQUE,
    serie VARCHAR(10) NOT NULL,
    numero_folio INT NOT NULL,
    cliente_id UUID NOT NULL REFERENCES clientes(id),
    evento_id UUID REFERENCES eventos(id),
    fecha_emision DATE NOT NULL,
    tipo_pago VARCHAR(15) NOT NULL, -- CONTADO, CREDITO
    moneda VARCHAR(3) NOT NULL DEFAULT 'NIO', -- NIO, USD
    tasa_cambio_bcn NUMERIC(10,4) NOT NULL DEFAULT 1.0000,
    subtotal_gravado NUMERIC(14,4) NOT NULL DEFAULT 0.0000,
    subtotal_exento NUMERIC(14,4) NOT NULL DEFAULT 0.0000,
    monto_iva NUMERIC(14,4) NOT NULL DEFAULT 0.0000, -- IVA 15%
    total NUMERIC(14,4) NOT NULL DEFAULT 0.0000,
    monto_letras TEXT NOT NULL, -- Conversión automática a letras
    saldo_pendiente NUMERIC(14,4) NOT NULL DEFAULT 0.0000,
    estado VARCHAR(15) NOT NULL DEFAULT 'EMITIDA', -- EMITIDA, ANULADA
    observaciones TEXT,
    creado_en TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_facturas_correlativo ON facturas(correlativo_preimpreso);
CREATE INDEX idx_facturas_fecha ON facturas(fecha_emision);
CREATE INDEX idx_facturas_cliente ON facturas(cliente_id);
CREATE INDEX idx_facturas_evento ON facturas(evento_id);

CREATE TABLE IF NOT EXISTS factura_detalles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    factura_id UUID NOT NULL REFERENCES facturas(id) ON DELETE CASCADE,
    descripcion VARCHAR(255) NOT NULL,
    cantidad NUMERIC(10,2) NOT NULL DEFAULT 1.00,
    precio_unitario NUMERIC(14,4) NOT NULL DEFAULT 0.0000,
    exento BOOLEAN NOT NULL DEFAULT FALSE,
    subtotal NUMERIC(14,4) NOT NULL DEFAULT 0.0000,
    monto_iva NUMERIC(14,4) NOT NULL DEFAULT 0.0000,
    total_linea NUMERIC(14,4) NOT NULL DEFAULT 0.0000
);

-- ------------------------------------------------------------------------------
-- MÓDULO 3: CUOTAS DE CRÉDITO Y RECAUDOS (CxC)
-- ------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS facturas_cuotas (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    factura_id UUID NOT NULL REFERENCES facturas(id) ON DELETE CASCADE,
    numero_cuota INT NOT NULL,
    fecha_vence DATE NOT NULL,
    monto NUMERIC(14,4) NOT NULL DEFAULT 0.0000,
    saldo NUMERIC(14,4) NOT NULL DEFAULT 0.0000,
    estado VARCHAR(15) NOT NULL DEFAULT 'PENDIENTE', -- PENDIENTE, PAGADA, PARCIAL
    CONSTRAINT uq_factura_cuota UNIQUE (factura_id, numero_cuota)
);

CREATE INDEX idx_cuotas_vence ON facturas_cuotas(fecha_vence) WHERE estado != 'PAGADA';

CREATE TABLE IF NOT EXISTS recibos_caja (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    numero_recibo VARCHAR(30) NOT NULL UNIQUE,
    cliente_id UUID NOT NULL REFERENCES clientes(id),
    fecha DATE NOT NULL,
    moneda VARCHAR(3) NOT NULL DEFAULT 'NIO',
    tasa_cambio NUMERIC(10,4) NOT NULL DEFAULT 1.0000,
    total_recibido NUMERIC(14,4) NOT NULL DEFAULT 0.0000,
    estado VARCHAR(15) NOT NULL DEFAULT 'EMITIDO', -- EMITIDO, ANULADO
    notas TEXT,
    creado_en TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS recibo_caja_detalles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    recibo_id UUID NOT NULL REFERENCES recibos_caja(id) ON DELETE CASCADE,
    cuota_id UUID NOT NULL REFERENCES facturas_cuotas(id),
    monto_aplicado NUMERIC(14,4) NOT NULL DEFAULT 0.0000
);

CREATE TABLE IF NOT EXISTS retenciones_recibidas (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    recibo_id UUID NOT NULL REFERENCES recibos_caja(id) ON DELETE CASCADE,
    factura_id UUID NOT NULL REFERENCES facturas(id),
    tipo_retencion VARCHAR(20) NOT NULL, -- IR_2, IR_1, ALMA_1
    numero_comprobante VARCHAR(50) NOT NULL,
    base_imponible NUMERIC(14,4) NOT NULL,
    porcentaje NUMERIC(5,2) NOT NULL,
    monto_retenido NUMERIC(14,4) NOT NULL,
    creado_en TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- ------------------------------------------------------------------------------
-- MÓDULO 4: CARTERA DE PROVEEDORES (CxP, Gastos por Evento y Retenciones)
-- ------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS facturas_proveedor (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    proveedor_id UUID NOT NULL REFERENCES proveedores(id),
    evento_id UUID REFERENCES eventos(id), -- Vinculación a evento (Centro de Costos)
    numero_factura VARCHAR(50) NOT NULL,
    fecha_factura DATE NOT NULL,
    fecha_vence DATE NOT NULL,
    moneda VARCHAR(3) NOT NULL DEFAULT 'NIO',
    tasa_cambio NUMERIC(10,4) NOT NULL DEFAULT 1.0000,
    subtotal NUMERIC(14,4) NOT NULL DEFAULT 0.0000,
    monto_iva NUMERIC(14,4) NOT NULL DEFAULT 0.0000,
    total NUMERIC(14,4) NOT NULL DEFAULT 0.0000,
    saldo NUMERIC(14,4) NOT NULL DEFAULT 0.0000,
    estado VARCHAR(15) NOT NULL DEFAULT 'PENDIENTE',
    creado_en TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_prov_factura UNIQUE (proveedor_id, numero_factura)
);

CREATE TABLE IF NOT EXISTS pagos_proveedor (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    proveedor_id UUID NOT NULL REFERENCES proveedores(id),
    fecha DATE NOT NULL,
    moneda VARCHAR(3) NOT NULL DEFAULT 'NIO',
    tasa_cambio NUMERIC(10,4) NOT NULL DEFAULT 1.0000,
    total_pagado NUMERIC(14,4) NOT NULL DEFAULT 0.0000,
    referencia_pago VARCHAR(50),
    creado_en TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS retenciones_efectuadas (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    pago_proveedor_id UUID NOT NULL REFERENCES pagos_proveedor(id) ON DELETE CASCADE,
    factura_proveedor_id UUID NOT NULL REFERENCES facturas_proveedor(id),
    tipo_retencion VARCHAR(30) NOT NULL, -- IR_2_SERVICIOS_GRALES (2%), IR_10_PROFESIONALES (10%)
    numero_comprobante VARCHAR(50) NOT NULL UNIQUE,
    base_imponible NUMERIC(14,4) NOT NULL,
    porcentaje NUMERIC(5,2) NOT NULL,
    monto_retenido NUMERIC(14,4) NOT NULL,
    creado_en TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- ------------------------------------------------------------------------------
-- MÓDULO 5: CONTABILIDAD (Libro Diario y Partida Doble)
-- ------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS asientos_contables (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    numero_asiento VARCHAR(30) NOT NULL UNIQUE,
    fecha DATE NOT NULL,
    concepto TEXT NOT NULL,
    origen VARCHAR(30) NOT NULL, -- FACTURA_EMISION, RECAUDO_CUOTA, FACTURA_ANULACION, PAGO_PROVEEDOR, GASTO_PROVEEDOR
    referencia_id UUID,
    creado_en TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_asientos_fecha ON asientos_contables(fecha);

CREATE TABLE IF NOT EXISTS asiento_lineas (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    asiento_id UUID NOT NULL REFERENCES asientos_contables(id) ON DELETE CASCADE,
    cuenta_id UUID NOT NULL REFERENCES cuentas_contables(id),
    debe NUMERIC(14,4) NOT NULL DEFAULT 0.0000,
    haber NUMERIC(14,4) NOT NULL DEFAULT 0.0000,
    debe_usd NUMERIC(14,4) NOT NULL DEFAULT 0.0000,
    haber_usd NUMERIC(14,4) NOT NULL DEFAULT 0.0000,
    CONSTRAINT chk_debe_haber_positivo CHECK (debe >= 0 AND haber >= 0 AND debe_usd >= 0 AND haber_usd >= 0)
);

-- ------------------------------------------------------------------------------
-- DATOS SEMILLA
-- ------------------------------------------------------------------------------
INSERT INTO cuentas_contables (codigo, nombre, tipo, nivel) VALUES
('1101', 'Caja General', 'ACTIVO', 1),
('1102', 'Bancos Nacionales (NIO/USD)', 'ACTIVO', 1),
('1103', 'Cuentas por Cobrar Clientes', 'ACTIVO', 1),
('1104', 'Anticipo de IR Retenido por Clientes (2%/1%)', 'ACTIVO', 1),
('1105', 'Anticipo Retención Municipal ALMA (1%)', 'ACTIVO', 1),
('2101', 'Cuentas por Pagar Proveedores', 'PASIVO', 1),
('2102', 'Débito Fiscal IVA 15% (Por Pagar)', 'PASIVO', 1),
('2103', 'Retenciones IR por Pagar a Proveedores (2%/10%)', 'PASIVO', 1),
('4101', 'Ingresos por Eventos Corporativos y Talleres', 'INGRESO', 1),
('5101', 'Costos Operativos de Eventos', 'GASTO', 1)
ON CONFLICT (codigo) DO NOTHING;

-- Usuario Admin por Defecto
INSERT INTO usuarios (nombre, email, password_hash, rol) VALUES
('Administrador SIFACO', 'admin@sifaco.ni', '$2a$10$7EqJtq986P2B2Xn83b5e4.1.1.1.1.1.1.1.1.1.1.1.1.1.1', 'ADMIN')
ON CONFLICT (email) DO NOTHING;

-- Talonario Demo DGI
INSERT INTO talonarios_dgi (numero_autorizacion, serie, numero_desde, numero_hasta, numero_actual, fecha_autorizacion, fecha_vencimiento) VALUES
('AUT-DGI-2026-9988', 'A', 1, 1000, 1, '2026-01-01', '2026-12-31')
ON CONFLICT (numero_autorizacion) DO NOTHING;
