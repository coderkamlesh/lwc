# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM golang:1.25.4-alpine AS build

WORKDIR /src

# Keep dependency resolution in its own layer for fast rebuilds.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
	go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH

RUN --mount=type=cache,target=/go/pkg/mod \
	CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
	go build \
		-tags lambda.norpc \
		-trimpath \
		-ldflags="-s -w" \
		-o /out/bootstrap \
		./cmd/app

FROM public.ecr.aws/lambda/provided:al2023

# Lambda's custom Go runtime starts this executable for every invocation.
COPY --from=build /out/bootstrap /var/task/bootstrap

ENTRYPOINT ["/var/task/bootstrap"]
