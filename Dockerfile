# Stage 1 of 2 [build rowi]
FROM golang:alpine

ENV GOPATH /go
RUN apk add --update git bash
ADD . /go/src/github.com/damonpetta/rowi
WORKDIR /go/src/github.com/damonpetta/rowi
RUN go get -u github.com/gobuffalo/packr/... && \
    packr build

# Stage 2 of 2 [copy assets to thin image]
FROM alpine:edge

# Copy rowi from builder
RUN apk add --update git bash
ENV GIN_MODE release
COPY --from=0 /go/src/github.com/damonpetta/rowi/rowi /usr/bin/rowi
ADD entrypoint.sh /app/entrypoint.sh
WORKDIR /app
EXPOSE 8000
ENTRYPOINT ["/app/entrypoint.sh"]

