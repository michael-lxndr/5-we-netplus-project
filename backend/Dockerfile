FROM golang:1.24-bookworm AS development

# Air recompila y reinicia la API cada vez que se guarda un archivo .go
RUN go install github.com/air-verse/air@v1.61.7

WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .

EXPOSE 8080
CMD ["air", "-c", ".air.toml"]
