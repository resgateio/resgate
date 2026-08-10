FROM --platform=$BUILDPLATFORM golang:1.22.4-alpine3.20 as builder

LABEL maintainer="Samuel Jirenius <samuel@jirenius.com>"

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src/resgate

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -v -ldflags "-s -w" -o /resgate

FROM scratch
COPY --from=builder /resgate /resgate
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

EXPOSE 8080

ENTRYPOINT ["/resgate"]
CMD [""]
