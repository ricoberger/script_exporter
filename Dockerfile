FROM golang:1.25.5 AS build
WORKDIR /script_exporter
COPY go.mod go.sum /script_exporter/
RUN go mod download
COPY . .
RUN export CGO_ENABLED=0 && make build

FROM alpine:3.23.2
RUN apk add --no-cache --update bash curl jq ca-certificates tini python3
RUN mkdir /script_exporter
COPY --from=build /script_exporter/bin/script_exporter /script_exporter
WORKDIR /script_exporter
USER nobody
ENTRYPOINT  [ "/sbin/tini", "--", "/script_exporter/script_exporter" ]
