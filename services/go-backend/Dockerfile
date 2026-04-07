FROM golang:1.25.0-alpine AS builder

WORKDIR /src
COPY services/go-backend/go.mod services/go-backend/go.sum /src/
COPY services/go-backend/main.go /src/main.go
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/go-backend ./main.go

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /out/go-backend /app/go-backend
EXPOSE 8080
CMD ["/app/go-backend"]
