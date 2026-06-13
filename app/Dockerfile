# syntax=docker/dockerfile:1
#
# Multi-stage build for the Phase-1 sample app (ADR-004): a statically-linked,
# std-lib-only Go binary copied into a scratch image for the smallest possible
# footprint and fastest cold start (the scale-to-zero baseline).

# ---- build stage ----
FROM golang:1.26-alpine AS build
WORKDIR /src

# Cache module resolution separately from source for faster rebuilds.
COPY go.mod ./
RUN go mod download

COPY . .

# CGO disabled + static link so the binary runs on scratch (no libc).
# -trimpath strips local paths; -ldflags "-s -w" drops debug info for a smaller binary.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath -ldflags="-s -w" \
    -o /out/app .

# ---- final stage ----
FROM scratch

# Run as a non-root UID (distroless "nonroot" convention) for least privilege.
USER 65532:65532

COPY --from=build /out/app /app

EXPOSE 8080
ENTRYPOINT ["/app"]
