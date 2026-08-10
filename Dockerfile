# 1. Derleme Aşaması (Build Stage)
FROM golang:alpine AS builder

WORKDIR /app

# Bağımlılıkları önbelleğe almak için go.mod ve go.sum kopyalanır
COPY go.mod go.sum ./
RUN go mod download

# Tüm proje kodları kopyalanır ve CGO kapalı şekilde derlenir
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# 2. Çalıştırma Aşaması (Production Stage - Hafif Alpine İmajı)
FROM alpine:latest

WORKDIR /app

# Derlenen binary dosyasını kopyala
COPY --from=builder /app/main .
COPY --from=builder /app/public ./public

EXPOSE 8080

CMD ["./main"]