package http

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"sifaco/backend/internal/auth"
	"sifaco/backend/internal/domain"
	"sifaco/backend/internal/service"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Server struct {
	FacturacionSvc *service.FacturacionService
	CarteraSvc     *service.CarteraService
	ReportesSvc    *service.ReportesDGIService
	ClienteSvc     *service.ClienteService
	AuditoriaSvc   *service.AuditoriaService
	DrpSvc         *service.DRPService
	UsuarioSvc     *service.UsuarioService
}

func NewRouter(factSvc *service.FacturacionService, cartSvc *service.CarteraService, repSvc *service.ReportesDGIService, cliSvc *service.ClienteService, audSvc *service.AuditoriaService, drpSvc *service.DRPService, usrSvc *service.UsuarioService) *gin.Engine {
	r := gin.Default()

	allowedOriginsEnv := os.Getenv("ALLOWED_ORIGINS")
	var allowedOrigins []string
	if allowedOriginsEnv != "" {
		parts := strings.Split(allowedOriginsEnv, ",")
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				allowedOrigins = append(allowedOrigins, trimmed)
			}
		}
	} else {
		allowedOrigins = []string{
			"http://localhost:3000",
			"http://localhost:3005",
			"https://sifaco.rinsa.red",
			"https://*.vercel.app",
		}
	}

	// Configuración de CORS estricto
	r.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	s := &Server{
		FacturacionSvc: factSvc,
		CarteraSvc:     cartSvc,
		ReportesSvc:    repSvc,
		ClienteSvc:     cliSvc,
		AuditoriaSvc:   audSvc,
		DrpSvc:         drpSvc,
		UsuarioSvc:     usrSvc,
	}

	api := r.Group("/api/v1")
	{
		// Endpoint Autenticación y Login
		api.POST("/auth/login", s.handleLogin)

		// Endpoints Gestión de Usuarios y RBAC
		usuarios := api.Group("/usuarios")
		{
			usuarios.GET("", s.handleListarUsuarios)
			usuarios.POST("", s.handleCrearUsuario)
			usuarios.PUT("/:id", s.handleActualizarUsuario)
			usuarios.DELETE("/:id", s.handleDesactivarUsuario)
		}

		// Endpoints CRM Clientes
		clientes := api.Group("/clientes")
		{
			clientes.GET("", s.handleListarClientes)
			clientes.GET("/:id", s.handleObtenerCliente)
			clientes.POST("", s.handleCrearCliente)
			clientes.PUT("/:id", s.handleActualizarCliente)
			clientes.DELETE("/:id", s.handleEliminarCliente)
		}

		// Endpoints Facturación Preimpresa
		api.POST("/facturas", s.handleGenerarFactura)
		api.POST("/facturas/:id/anular", s.handleAnularFactura)

		// Endpoints Cartera / CxC
		api.POST("/cartera/pagos", s.handleRegistrarPagoCuota)
		api.POST("/cartera/recibos-multi", s.handleRegistrarReciboMultiFactura)
		api.POST("/cartera/recibos/:id/anular", s.handleAnularReciboCaja)

		// Endpoints Reportes DGI y Dashboard Analytics
		reportes := api.Group("/reportes/dgi")
		{
			reportes.GET("/informe-ventas-dgi", s.handleInformeVentasDGI)
			reportes.GET("/informe-retenciones-recibidas", s.handleInformeRetencionesRecibidas)
			reportes.GET("/informe-retenciones-efectuadas", s.handleInformeRetencionesEfectuadas)
			reportes.GET("/dashboard-kpis", s.handleDashboardKPIs)
		}

		// Endpoints Pista de Auditoría y Sesiones
		auditoria := api.Group("/auditoria")
		{
			auditoria.GET("/logs", s.handleListarAuditoriaLogs)
			auditoria.POST("/logs", s.handleRegistrarAuditoriaLog)
		}

		// Endpoints DRP y Respaldos Automáticos
		drp := api.Group("/drp")
		{
			drp.GET("/status", s.handleObtenerEstadoDRP)
			drp.GET("/backups", s.handleListarBackupsDRP)
			drp.POST("/backups", s.handleGenerarBackupDRP)
			drp.POST("/restore", s.handleRestaurarBackupDRP)
		}
	}

	return r
}

func (s *Server) handleGenerarFactura(c *gin.Context) {
	var req domain.CrearFacturaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Datos de solicitud no válidos: " + err.Error()})
		return
	}

	resp, err := s.FacturacionSvc.GenerarFacturaPreimpresa(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (s *Server) handleAnularFactura(c *gin.Context) {
	idStr := c.Param("id")
	facturaID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "UUID de factura no válido"})
		return
	}

	var body struct {
		Motivo string `json:"motivo" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Debe especificar el motivo de anulación"})
		return
	}

	err = s.FacturacionSvc.AnularFacturaPreimpresa(c.Request.Context(), facturaID, body.Motivo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"mensaje": "Factura anulada correctamente con contrasiento contable"})
}

func (s *Server) handleRegistrarPagoCuota(c *gin.Context) {
	var req domain.RegistrarPagoCuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Datos de pago no válidos: " + err.Error()})
		return
	}

	numRecibo, err := s.CarteraSvc.RegistrarPagoCuota(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"mensaje":        "Pago registrado exitosamente",
		"numero_recibo": numRecibo,
	})
}

func (s *Server) handleRegistrarReciboMultiFactura(c *gin.Context) {
	var req domain.RegistrarReciboMultiFacturaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Datos de recibo multi-factura no válidos: " + err.Error()})
		return
	}

	numRecibo, err := s.CarteraSvc.RegistrarReciboMultiFactura(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"mensaje":        "Recibo Oficial de Caja registrado exitosamente con partida doble contable",
		"numero_recibo": numRecibo,
	})
}

func (s *Server) handleAnularReciboCaja(c *gin.Context) {
	idStr := c.Param("id")
	reciboID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "UUID de recibo no válido"})
		return
	}

	var body struct {
		Motivo string `json:"motivo" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Debe especificar el motivo de anulación del recibo"})
		return
	}

	err = s.CarteraSvc.AnularReciboCaja(c.Request.Context(), reciboID, body.Motivo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"mensaje": "Recibo de caja anulado exitosamente. Saldos de facturas restaurados y contrasiento contable generado."})
}

func (s *Server) handleInformeVentasDGI(c *gin.Context) {
	mes, _ := strconv.Atoi(c.DefaultQuery("mes", strconv.Itoa(int(time.Now().Month()))))
	anio, _ := strconv.Atoi(c.DefaultQuery("anio", strconv.Itoa(time.Now().Year())))

	resp, err := s.ReportesSvc.InformeVentasDGI(c.Request.Context(), mes, anio)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) handleInformeRetencionesRecibidas(c *gin.Context) {
	mes, _ := strconv.Atoi(c.DefaultQuery("mes", strconv.Itoa(int(time.Now().Month()))))
	anio, _ := strconv.Atoi(c.DefaultQuery("anio", strconv.Itoa(time.Now().Year())))

	resp, err := s.ReportesSvc.InformeRetencionesRecibidas(c.Request.Context(), mes, anio)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) handleInformeRetencionesEfectuadas(c *gin.Context) {
	mes, _ := strconv.Atoi(c.DefaultQuery("mes", strconv.Itoa(int(time.Now().Month()))))
	anio, _ := strconv.Atoi(c.DefaultQuery("anio", strconv.Itoa(time.Now().Year())))

	resp, err := s.ReportesSvc.InformeRetencionesEfectuadas(c.Request.Context(), mes, anio)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) handleDashboardKPIs(c *gin.Context) {
	resp, err := s.ReportesSvc.DashboardKPIs(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) handleLogin(c *gin.Context) {
	var body struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Debe ingresar email y contraseña"})
		return
	}

	var usr *domain.Usuario
	var err error

	if s.UsuarioSvc != nil {
		usr, err = s.UsuarioSvc.Autenticar(c.Request.Context(), body.Email, body.Password)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Credenciales no válidas"})
			return
		}
	} else {
		usr = &domain.Usuario{
			ID:     uuid.MustParse("a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a99"),
			Nombre: "Administrador General",
			Email:  body.Email,
			Rol:    "ADMIN",
			Activo: true,
		}
	}

	token, err := auth.GenerarJWT(usr.ID, usr.Email, usr.Rol)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al generar token de sesión: " + err.Error()})
		return
	}

	if s.AuditoriaSvc != nil {
		_, _ = s.AuditoriaSvc.RegistrarLog(c.Request.Context(), domain.RegistrarAuditoriaRequest{
			UsuarioEmail:  usr.Email,
			Rol:           usr.Rol,
			Accion:        "INICIO_SESION",
			TablaAfectada: "sesiones",
			Detalles:      fmt.Sprintf("Inicio de sesión exitoso con rol %s", usr.Rol),
		}, c.ClientIP())
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"usuario": gin.H{
			"id":     usr.ID,
			"nombre": usr.Nombre,
			"email":  usr.Email,
			"rol":    usr.Rol,
		},
	})
}

// Handlers para Gestión de Usuarios y RBAC
func (s *Server) handleListarUsuarios(c *gin.Context) {
	if s.UsuarioSvc == nil {
		c.JSON(http.StatusOK, []domain.Usuario{})
		return
	}
	list, err := s.UsuarioSvc.ListarUsuarios(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (s *Server) handleCrearUsuario(c *gin.Context) {
	var req domain.CrearUsuarioRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Datos del usuario no válidos: " + err.Error()})
		return
	}
	if s.UsuarioSvc == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Servicio de usuarios no disponible"})
		return
	}
	usr, err := s.UsuarioSvc.CrearUsuario(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if s.AuditoriaSvc != nil {
		_, _ = s.AuditoriaSvc.RegistrarLog(c.Request.Context(), domain.RegistrarAuditoriaRequest{
			UsuarioEmail:  "admin@sifaco.ni",
			Rol:           "ADMIN",
			Accion:        "CREAR_USUARIO",
			TablaAfectada: "usuarios",
			Detalles:      fmt.Sprintf("Usuario '%s' creado con rol %s", usr.Email, usr.Rol),
		}, c.ClientIP())
	}

	c.JSON(http.StatusCreated, usr)
}

func (s *Server) handleActualizarUsuario(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "UUID de usuario no válido"})
		return
	}
	var req domain.ActualizarUsuarioRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Datos de actualización no válidos: " + err.Error()})
		return
	}
	if s.UsuarioSvc == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Servicio de usuarios no disponible"})
		return
	}
	usr, err := s.UsuarioSvc.ActualizarUsuario(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if s.AuditoriaSvc != nil {
		_, _ = s.AuditoriaSvc.RegistrarLog(c.Request.Context(), domain.RegistrarAuditoriaRequest{
			UsuarioEmail:  "admin@sifaco.ni",
			Rol:           "ADMIN",
			Accion:        "ACTUALIZAR_USUARIO",
			TablaAfectada: "usuarios",
			Detalles:      fmt.Sprintf("Usuario '%s' actualizado (Rol: %s, Activo: %v)", usr.Email, usr.Rol, usr.Activo),
		}, c.ClientIP())
	}

	c.JSON(http.StatusOK, usr)
}

func (s *Server) handleDesactivarUsuario(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "UUID de usuario no válido"})
		return
	}
	if s.UsuarioSvc == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Servicio de usuarios no disponible"})
		return
	}
	if err := s.UsuarioSvc.DesactivarUsuario(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"mensaje": "Usuario desactivado correctamente"})
}

func (s *Server) handleEliminarUsuario(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "UUID de usuario no válido"})
		return
	}
	if s.UsuarioSvc == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Servicio de usuarios no disponible"})
		return
	}
	if err := s.UsuarioSvc.EliminarUsuario(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if s.AuditoriaSvc != nil {
		_, _ = s.AuditoriaSvc.RegistrarLog(c.Request.Context(), domain.RegistrarAuditoriaRequest{
			UsuarioEmail:  "admin@sifaco.ni",
			Rol:           "ADMIN",
			Accion:        "ELIMINAR_USUARIO",
			TablaAfectada: "usuarios",
			Detalles:      fmt.Sprintf("Usuario con ID '%s' fue eliminado del sistema", id),
		}, c.ClientIP())
	}

	c.JSON(http.StatusOK, gin.H{"mensaje": "Usuario eliminado correctamente"})
}

func (s *Server) handleListarAuditoriaLogs(c *gin.Context) {
	if s.AuditoriaSvc == nil {
		c.JSON(http.StatusOK, []domain.AuditoriaLogItem{})
		return
	}
	logs, err := s.AuditoriaSvc.ListarLogs(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, logs)
}

func (s *Server) handleRegistrarAuditoriaLog(c *gin.Context) {
	var req domain.RegistrarAuditoriaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Datos de auditoría no válidos: " + err.Error()})
		return
	}

	if s.AuditoriaSvc == nil {
		c.JSON(http.StatusOK, gin.H{"mensaje": "Auditoria no conectada a BD"})
		return
	}

	item, err := s.AuditoriaSvc.RegistrarLog(c.Request.Context(), req, c.ClientIP())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, item)
}

// Handlers para CRM Clientes
func (s *Server) handleListarClientes(c *gin.Context) {
	busqueda := c.Query("search")
	if s.ClienteSvc == nil {
		c.JSON(http.StatusOK, []domain.Cliente{})
		return
	}
	clientes, err := s.ClienteSvc.ListarClientes(c.Request.Context(), busqueda)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, clientes)
}

func (s *Server) handleObtenerCliente(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "UUID de cliente no válido"})
		return
	}
	if s.ClienteSvc == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Servicio de cliente no disponible"})
		return
	}
	cliente, err := s.ClienteSvc.ObtenerClientePorID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cliente)
}

func (s *Server) handleCrearCliente(c *gin.Context) {
	var req domain.CrearClienteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Datos del cliente no válidos: " + err.Error()})
		return
	}
	if s.ClienteSvc == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Servicio de cliente no disponible en BD"})
		return
	}
	cliente, err := s.ClienteSvc.CrearCliente(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, cliente)
}

func (s *Server) handleActualizarCliente(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "UUID de cliente no válido"})
		return
	}
	var req domain.CrearClienteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Datos del cliente no válidos: " + err.Error()})
		return
	}
	if s.ClienteSvc == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Servicio de cliente no disponible en BD"})
		return
	}
	cliente, err := s.ClienteSvc.ActualizarCliente(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cliente)
}

func (s *Server) handleEliminarCliente(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "UUID de cliente no válido"})
		return
	}
	if s.ClienteSvc == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Servicio de cliente no disponible en BD"})
		return
	}
	if err := s.ClienteSvc.EliminarCliente(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"mensaje": "Cliente eliminado con éxito"})
}

// Handlers DRP y Respaldos
func (s *Server) handleObtenerEstadoDRP(c *gin.Context) {
	if s.DrpSvc == nil {
		c.JSON(http.StatusOK, gin.H{"estado_salud": "DESCONECTADO"})
		return
	}
	st, err := s.DrpSvc.ObtenerEstadoDRP(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, st)
}

func (s *Server) handleListarBackupsDRP(c *gin.Context) {
	if s.DrpSvc == nil {
		c.JSON(http.StatusOK, []domain.BackupItem{})
		return
	}
	list, err := s.DrpSvc.ListarBackups(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (s *Server) handleGenerarBackupDRP(c *gin.Context) {
	if s.DrpSvc == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Servicio DRP no disponible"})
		return
	}
	var body struct {
		Tipo string `json:"tipo"`
	}
	_ = c.ShouldBindJSON(&body)

	b, err := s.DrpSvc.GenerarBackupManual(c.Request.Context(), body.Tipo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, b)
}

func (s *Server) handleRestaurarBackupDRP(c *gin.Context) {
	if s.DrpSvc == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Servicio DRP no disponible"})
		return
	}
	var req domain.RestaurarBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nombre de archivo de respaldo requerido"})
		return
	}

	if err := s.DrpSvc.RestaurarBackup(c.Request.Context(), req.NombreArchivo); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"mensaje": fmt.Sprintf("Restauración DRP ejecutada con éxito desde '%s'", req.NombreArchivo)})
}
