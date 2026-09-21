FROM golang:1.24-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /uml-exporter .

FROM alpine:3.21

LABEL org.opencontainers.image.source="https://github.com/EastArctica/uml-campus-map-exporter"
LABEL org.opencontainers.image.description="Prometheus exporter for UMass Lowell transit and parking data"

RUN apk add --no-cache ca-certificates && adduser -D -H -u 65532 exporter
COPY --from=build /uml-exporter /usr/local/bin/uml-exporter

USER exporter
EXPOSE 9828
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 CMD wget -q -O /dev/null http://127.0.0.1:9828/-/healthy || exit 1
ENTRYPOINT ["/usr/local/bin/uml-exporter"]
