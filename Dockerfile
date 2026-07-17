# The manager binary is built by `make build` (dist/manager-<arch>); this
# image only packages it. Multi-arch builds work by supplying a binary per
# target platform: GOARCH=amd64 make build && GOARCH=arm64 make build.
FROM gcr.io/distroless/static:nonroot
ARG TARGETARCH
WORKDIR /
COPY ./dist/manager-${TARGETARCH} /manager
USER 65532:65532

ENTRYPOINT ["/manager"]
