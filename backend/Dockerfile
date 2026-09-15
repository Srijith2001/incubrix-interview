# syntax=docker/dockerfile:1

# ---- build ----
FROM golang:1.26-alpine AS build

WORKDIR /src

# Dependencies are their own layer, so editing source does not re-download them.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# TARGETOS and TARGETARCH are supplied by buildx for multi-arch builds. They are
# empty under a plain `docker build`, which leaves Go on its own defaults.
ARG TARGETOS
ARG TARGETARCH

# CGO_ENABLED=0 produces a static binary, which is what lets the runtime stage
# be an image with no libc at all. -trimpath keeps build paths out of it, and
# -s -w drop the symbol and DWARF tables.
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ---- run ----
# distroless static carries CA certificates, which the frankfurter.dev TLS call
# needs, and nothing else: no shell, no package manager, no libc.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/server /server

# uid 65532, baked into the base image. Declared here so it survives a
# `docker run --user` default and is visible to anyone reading this file.
USER nonroot:nonroot

EXPOSE 8080

# Exec form, so the server is PID 1 and receives the SIGTERM that `docker stop`
# sends. Its graceful shutdown depends on that signal arriving.
ENTRYPOINT ["/server"]
