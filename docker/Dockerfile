FROM golang:1.20-alpine as builder

LABEL maintainer="Samuel Jirenius <samuel@jirenius.com>"

WORKDIR /src/resgate

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -ldflags "-s -w" -o /resgate

FROM scratch

LABEL org.opencontainers.image.source="https://github.com/VoIPGRID/resgate"

COPY --from=builder /resgate /resgate
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

EXPOSE 8080

ENTRYPOINT ["/resgate"]
CMD [""]
