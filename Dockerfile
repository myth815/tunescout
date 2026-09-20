FROM --platform=$BUILDPLATFORM golang:1.24-bookworm AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go test ./... && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/tunescout ./cmd/tunescout

FROM --platform=$BUILDPLATFORM alpine:3.22 AS fpcalc
ARG TARGETARCH
ARG CHROMAPRINT_VERSION=1.6.1
RUN case "$TARGETARCH" in amd64) archive_arch=x86_64 ;; arm64) archive_arch=arm64 ;; *) exit 1 ;; esac && \
    wget -qO- "https://github.com/acoustid/chromaprint/releases/download/v${CHROMAPRINT_VERSION}/chromaprint-fpcalc-${CHROMAPRINT_VERSION}-linux-${archive_arch}.tar.gz" \
      | tar -xz --strip-components=1 -C /usr/local/bin

FROM gcr.io/distroless/cc-debian12:nonroot
LABEL org.opencontainers.image.title="TuneScout" \
      org.opencontainers.image.description="Stateless federated music search with provider provenance" \
      org.opencontainers.image.source="https://github.com/myth815/tunescout" \
      org.opencontainers.image.licenses="MIT"
COPY --from=build /out/tunescout /usr/local/bin/tunescout
COPY --from=fpcalc /usr/local/bin/fpcalc /usr/local/bin/fpcalc
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/usr/local/bin/tunescout", "healthcheck"]
ENTRYPOINT ["/usr/local/bin/tunescout"]
