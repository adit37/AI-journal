# Gunakan golang versi alpine yang ringan sebagai tahap builder
FROM golang:alpine AS builder

# Set working directory di dalam container
WORKDIR /app

# Copy file go.mod dan go.sum terlebih dahulu untuk caching dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy seluruh source code ke dalam container
COPY . .

# Build aplikasi Go dengan nama 'ai-journal'
# CGO_ENABLED=0 agar library C (cgo) dinonaktifkan, sehingga binary berjalan standalone di alpine
RUN CGO_ENABLED=0 GOOS=linux go build -o /ai-journal main.go

# --- TAHAP KEDUA (Image yang sangat ringan) ---
FROM alpine:latest

# Install sertifikat CA untuk komunikasi HTTPS (misal ke Google Gemini API)
RUN apk --no-cache add ca-certificates tzdata

# Set working directory di container akhir
WORKDIR /app

# Copy binary dari tahap builder
COPY --from=builder /ai-journal .
# Copy index.html agar UI bisa di-serve oleh web server Go
COPY --from=builder /app/index.html .

# Ekspos port 8080 (sesuai yang ada di main.go)
EXPOSE 8080

# Jalankan server
CMD ["./ai-journal"]
