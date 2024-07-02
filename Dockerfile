
# Build a go app into a minimal docker image with timezone support and SSL cert chains.
FROM golang:latest@sha256:c8736b8dbf2b12c98bb0eeed91eef58ecef52b8c2bd49b8044531e8d8d8d58e8 as builder
ARG TARGETOS
ARG TARGETARCH
ARG BUILD_FLAGS=""

RUN mkdir -p $GOPATH/pkg/mod $GOPATH/bin $GOPATH/src /build
COPY . /build
WORKDIR /build

RUN apt update \
    && apt install -y tzdata \
    && go generate ./... \
    && GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 \
       go build ${BUILD_FLAGS} -o /fogwillow .

FROM scratch
COPY --from=builder /fogwillow /fogwillow
# Make sure we have an ssl cert chain and timezone data.
# COPY --from=builder /etc/ssl /etc/ssl
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

ENV TZ=UTC

ENTRYPOINT [ "/fogwillow" ]