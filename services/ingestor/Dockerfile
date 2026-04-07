FROM golang:1.25.0-alpine AS build

WORKDIR /src
COPY services/ingestor/go.mod /src/go.mod
RUN go mod download
COPY services/ingestor /src
RUN go build -o /out/ingestor .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/ingestor /app/ingestor
CMD ["/app/ingestor"]
