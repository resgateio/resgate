FROM golang

WORKDIR $GOPATH/src/github.com/resgateio/resgate
COPY . .

RUN go get -d -v
RUN CGO_ENABLED=0 GO111MODULE=off go install -v -ldflags "-s -w"

EXPOSE 8080

ENTRYPOINT ["resgate"]
CMD ["--help"]
