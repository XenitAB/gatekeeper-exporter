FROM golang:1.15-alpine as builder
WORKDIR /workspace
RUN apk add --no-cache ca-certificates curl
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download
COPY main.go main.go
RUN CGO_ENABLED=0 go build -a -o gatekeeper-exporter main.go

FROM alpine:3.12
LABEL org.opencontainers.image.source="https://github.com/xenitab/gatekeeper-exporter"
RUN apk add --no-cache ca-certificates tini
COPY --from=builder /workspace/gatekeeper-exporter /usr/local/bin/
# Create minimal nsswitch.conf file to prioritize the usage of /etc/hosts over DNS queries.
# https://github.com/gliderlabs/docker-alpine/issues/367#issuecomment-354316460
RUN [ ! -e /etc/nsswitch.conf ] && echo 'hosts: files dns' > /etc/nsswitch.conf
RUN addgroup -S gatekeeper && adduser -S -g gatekeeper gatekeeper
USER gatekeeper
ENTRYPOINT [ "/sbin/tini", "--", "gatekeeper-exporter" ]
