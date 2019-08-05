FROM golang:1.12.5-alpine3.9 as builder

LABEL maintainer="Samuel Jirenius <samuel@jirenius.com>"

WORKDIR $GOPATH/src/github.com/resgateio/resgate

RUN apk update && apk add --no-cache git ca-certificates && update-ca-certificates
RUN mkdir /user && \
	echo 'nobody:x:65534:65534:nobody:/:' > /user/passwd && \
	echo 'nobody:x:65534:' > /user/group

COPY . .

RUN go get -d -v
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=off go build -v -ldflags "-s -w" -o /resgate

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /user/group /user/passwd /etc/
COPY --from=builder /resgate /resgate

EXPOSE 8080

USER nobody:nobody

ENTRYPOINT ["/resgate"]
CMD ["--config", "config.json"]
