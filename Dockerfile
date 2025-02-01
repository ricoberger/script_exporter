FROM golang:1.23.5 as build
RUN mkdir /build
WORKDIR /build
COPY . .
RUN export CGO_ENABLED=0 && make build

FROM alpine:3.21.2
RUN apk add --no-cache --update bash curl jq ca-certificates tini python3
COPY --from=build /build/bin/script_exporter /bin/script_exporter
EXPOSE 9469
ENTRYPOINT ["/sbin/tini", "--", "/bin/script_exporter"]
