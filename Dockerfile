FROM golang:1.20-alpine AS build_base
WORKDIR /tmp/speedtest-exporter

ARG VERSION="devel"

COPY . .

RUN apk --update add ca-certificates
RUN go mod tidy && \
    go mod vendor && \
    CGO_ENABLED=0 go build \
        -ldflags="-s -w -X main.version=${VERSION}" \
        -o ./out/speedtest-exporter \
         cmd/speedtest-exporter/speedtest-exporter.go

FROM scratch
COPY --from=build_base /tmp/speedtest-exporter/out/speedtest-exporter /bin/speedtest-exporter
COPY --from=build_base /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
EXPOSE 8080/tcp
ENTRYPOINT ["/bin/speedtest-exporter"]