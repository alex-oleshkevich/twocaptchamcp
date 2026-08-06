# syntax=docker/dockerfile:1.7

FROM golang:1.26.5-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN GOTOOLCHAIN=local go mod download

COPY . .

ARG VERSION=dev

RUN CGO_ENABLED=0 GOTOOLCHAIN=local go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/twocap ./cmd/twocap

FROM alpine:3.22

RUN addgroup -S twocap && adduser -S -G twocap twocap

COPY --from=build /out/twocap /usr/local/bin/twocap

ENV TWOCAPTCHAMCP_ADDRESS=0.0.0.0:8080

EXPOSE 8080

USER twocap
ENTRYPOINT ["/usr/local/bin/twocap"]
CMD ["mcp"]

HEALTHCHECK --interval=10s --timeout=5s --start-period=5s --retries=6 \
    CMD wget -q -O - http://127.0.0.1:8080/healthz >/dev/null
