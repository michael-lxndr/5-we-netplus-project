FROM golang:1.24-bookworm AS development

WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .

EXPOSE 8080
CMD ["go", "run", "./cmd/api"]
