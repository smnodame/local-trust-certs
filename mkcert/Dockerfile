FROM golang:1.15.7-alpine

RUN mkdir -p $$GOPATH/bin

ADD ./mkcert /go/src/mkcert
WORKDIR /go/src/mkcert

RUN go mod vendor

# RUN go build main.go
