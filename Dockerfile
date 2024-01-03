FROM golang:1.21.5-alpine3.18 as build
RUN apk update && apk add git make
RUN mkdir /build
WORKDIR /build
COPY . .
RUN export CGO_ENABLED=0 && make build

FROM alpine:3.19.0
RUN apk add --no-cache --update bash curl jq ca-certificates tini
COPY --from=build /build/bin/script_exporter /bin/script_exporter
EXPOSE 9469
ENTRYPOINT ["/sbin/tini", "--", "/bin/script_exporter"]
