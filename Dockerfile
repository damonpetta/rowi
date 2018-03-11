FROM alpine:edge

RUN apk add --update --virtual build-deps build-base go \
    && apk add --update git bash \
    && apk del build-deps \
    && rm -rf /var/cache/apk/*

ADD entrypoint.sh /entrypoint.sh

WORKDIR /app

EXPOSE 3000

ENTRYPOINT ["/entrypoint.sh"]
