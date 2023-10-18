FROM golang:alpine AS builder

MAINTAINER Osiloke Emoekpere ( me@osiloke.com ) 

RUN apk update \
	&& apk add -U git \
	&& apk add ca-certificates \
	&& go get -v github.com/mmatczuk/go-http-tunnel/cmd/tunneld \
	&& rm -rf /var/cache/apk/* 

# final stage
FROM alpine

WORKDIR /

RUN apk update && apk add openssl \
	&& apk add ca-certificates \
	&& rm -rf /var/cache/apk/* 

COPY --from=builder /go/bin/tunneld .

# default variables
ENV COUNTY "US"
ENV STATE "New Jersey"
ENV LOCATION "Piscataway"
ENV ORGANISATION "Ecample"
ENV ROOT_CN "Root"
ENV ISSUER_CN "Example Ltd"
ENV PUBLIC_CN "example.com"
ENV ROOT_NAME "root"
ENV ISSUER_NAME "example"
ENV PUBLIC_NAME "public"
ENV RSA_KEY_NUMBITS "2048"
ENV DAYS "365"

# certificate directories
ENV CERT_DIR "/etc/ssl/certs"

VOLUME ["$CERT_DIR"]

COPY *.ext /
COPY entrypoint.sh / 
COPY tunneld.sh / 

ENTRYPOINT [ "/entrypoint.sh" ]