# syntax=docker/dockerfile:1

# sluice: `sluice` CLI + HTTP service on Slurm clusters.
# Build:  docker build --build-arg VERSION=$(git rev-parse --short HEAD) -t <user>/sluice:latest .
# Run:    docker run --rm <user>/sluice:latest version
#         docker run --rm -v "$PWD:/work" -w /work <user>/sluice:latest it -manifest manifest.yaml

ARG GO_VERSION=1.26

FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X main.Version=${VERSION}" \
      -o /out/sluice ./cmd/sluice

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=build /out/sluice /usr/local/bin/sluice
ENTRYPOINT ["sluice"]
CMD ["version"]
