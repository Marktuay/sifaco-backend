# Estapa 1: Build de la aplicación Go
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copiar archivos de dependencias
COPY go.mod go.sum ./
RUN go mod download

# Copiar código fuente
COPY . .

# Compilar binario de producción optimizado para Linux
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/sifaco_api ./cmd/api

# Etapa 2: Imagen final ultra reducida para ejecución en Cloud Run o Docker Container
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata
ENV TZ=America/Managua

WORKDIR /root/

COPY --from=builder /app/sifaco_api .

EXPOSE 8080

CMD ["./sifaco_api"]
