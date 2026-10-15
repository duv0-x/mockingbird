# mockingbird container image (SPEC.md §10, T6).
#
# Build:
#   docker build --build-arg VERSION=$(git describe --tags --always --dirty) \
#                -t mockingbird:latest .
# Run:
#   docker run -p 8080:8080 -v mockingbird-data:/data mockingbird:latest
#
# The *_templ.go files are committed and CI rejects a stale tree, so the
# builder does not need the templ binary.

# Build stage: Go 1.26 toolchain, static binary from the committed sources.
FROM golang:1.26 AS build
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# /data is pre-created for UID 65532 (the distroless nonroot user): the store
# only creates missing directories, it cannot chown an existing root-owned one.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" \
        -o /out/bin/mockingbird ./cmd/mockingbird \
    && mkdir -p /out/data \
    && chown -R 65532:65532 /out/data

# Runtime: distroless static (CA certificates + tzdata, no shell), nonroot.
# Port 8080, context path /castlemock and data dir /data keep CastleMock's
# public surface; the JSON store lives on the /data volume.
FROM gcr.io/distroless/static-debian12:nonroot
ENV MOCKINGBIRD_DATA_DIR=/data
COPY --from=build --chown=65532:65532 /out/bin/mockingbird /bin/mockingbird
COPY --from=build --chown=65532:65532 /out/data /data
EXPOSE 8080
VOLUME ["/data"]
USER nonroot
ENTRYPOINT ["/bin/mockingbird"]
CMD ["serve"]