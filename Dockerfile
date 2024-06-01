FROM golang:1.22.3-alpine3.20 as build
RUN apk update && apk add git make
RUN mkdir /build
WORKDIR /build
COPY . .
RUN export CGO_ENABLED=0 && make build

FROM alpine:3.20.0
RUN apk add --no-cache --update bash curl jq ca-certificates tini python3
COPY --from=build /build/bin/script_exporter /bin/script_exporter
EXPOSE 9469
ENTRYPOINT ["/sbin/tini", "--", "/bin/script_exporter"]
