# Build stage
FROM golang:latest AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o myapp .

# Run stage
FROM alpine:latest AS run

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=build /app/myapp ./myapp
COPY .env ./

CMD ["./myapp"]
