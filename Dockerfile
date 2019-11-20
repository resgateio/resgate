FROM golang

ENV GO111MODULE=on

WORKDIR /src/resgate

COPY . .

RUN CGO_ENABLED=0 go build -v -ldflags "-s -w" -o /resgate

EXPOSE 8080

ENTRYPOINT ["/resgate"]
CMD ["--help"]
