# The build stage stays on the builder's own architecture and cross-compiles,
# rather than running under QEMU once per target. Go cross-compiles a static
# binary for nothing, so emulation would buy an identical artefact at several
# minutes a platform.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build

# buildx supplies these per platform being built.
ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags='-s -w' -o /action-changelog .

# scratch because the binary needs nothing beside it: it is static, pure Go, and
# opens no files but the changelog it was pointed at.
FROM scratch
COPY --from=build /action-changelog /action-changelog
ENTRYPOINT ["/action-changelog"]
