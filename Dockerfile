FROM golang:1.19.4-alpine3.17 as build
RUN apk update && apk add git make
RUN mkdir /build
WORKDIR /build
COPY . .
RUN export CGO_ENABLED=0 && make build

FROM alpine:3.17.0
RUN apk add --no-cache --update bash curl ca-certificates
COPY --from=build /build/bin/script_exporter /bin/script_exporter
EXPOSE 9469
ENTRYPOINT  [ "/bin/script_exporter" ]
