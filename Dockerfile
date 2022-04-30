FROM golang:1.18.1-alpine3.15 as build

RUN apk add --no-cache --update git make

RUN mkdir /build
WORKDIR /build
COPY . .
RUN make build


FROM alpine:3.15.4

LABEL maintainer="Rico Berger"
LABEL git.url="https://github.com/ricoberger/script_exporter"

RUN apk add --no-cache --update bash curl ca-certificates

USER nobody

COPY --from=build /build/bin/script_exporter /bin/script_exporter
EXPOSE 9469

ENTRYPOINT  [ "/bin/script_exporter" ]
