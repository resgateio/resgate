FROM golang:1.13.1-alpine3.10 as builder

LABEL maintainer="Samuel Jirenius <samuel@jirenius.com>"

ENV GO111MODULE=on

WORKDIR /src/resgate

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -ldflags "-s -w" -o /resgate

FROM scratch
COPY --from=builder /resgate /resgate

EXPOSE 8080

ENTRYPOINT ["/resgate"]
CMD [""]
