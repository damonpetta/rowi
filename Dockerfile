FROM golang:alpine

ENV GOPATH /go
EXPOSE 8000

RUN apk add --update git bash

ADD . /go/src/github.com/rowi
ADD entrypoint.sh /app/entrypoint.sh

WORKDIR /go/src/github.com/rowi

# ##TODO## Move to two stage build
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' . \
&& mv rowi /app/rowi \
    && rm -rf /var/cache/apk/* /go

WORKDIR /app

ENTRYPOINT ["/app/entrypoint.sh"]
