FROM golang:alpine

ENV GOPATH /go
EXPOSE 8000

RUN apk add --update git bash

ADD . /go/src/github.com/damonpetta/rowi

WORKDIR /go/src/github.com/damonpetta/rowi

# ##TODO## Move to two stage build
RUN go get -u github.com/gobuffalo/packr/... && \
    packr build

# Stage 2 of 2 [copy assets to thin image]
FROM alpine:edge

# Copy rowi from builder
RUN apk add --update git bash

COPY --from=0 /go/src/github.com/damonpetta/rowi/rowi /usr/bin/rowi

ADD entrypoint.sh /app/entrypoint.sh

WORKDIR /app

ENTRYPOINT ["/app/entrypoint.sh"]

