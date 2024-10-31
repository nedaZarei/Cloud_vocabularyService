FROM golang:1.23.2-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o englishpinglish

FROM scratch
COPY --from=builder /app/englishpinglish /app/englishpinglish

COPY --from=builder /app/config.yml /app/config.yml

EXPOSE 8080

ENTRYPOINT ["/app/englishpinglish"]