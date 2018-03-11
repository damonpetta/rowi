FROM golang:alpine

ENV GOPATH /go
EXPOSE 8000

RUN apk add --update git bash

ADD . /go/src/github.com/damonpetta/rowi
ADD entrypoint.sh /app/entrypoint.sh

WORKDIR /go/src/github.com/damonpetta/rowi

# ##TODO## Move to two stage build
RUN go build . \
    && mv rowi /app/rowi \
    && rm -rf /var/cache/apk/* /go

WORKDIR /app

ENTRYPOINT ["/app/entrypoint.sh"]
