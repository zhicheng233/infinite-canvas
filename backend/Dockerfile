FROM alpine:3.21 AS build-metadata
ARG BUILD_VERSION=
RUN apk add --no-cache git
COPY . /context
RUN /bin/sh /context/scripts/build-version.sh /context "${BUILD_VERSION}" > /build-version

FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend .
COPY --from=build-metadata /build-version /build-version
RUN BUILD_VERSION="$(cat /build-version)" && CGO_ENABLED=0 go build -ldflags "-X infinite-canvas-server/config.BuildVersion=${BUILD_VERSION}" -o /server .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai
COPY --from=builder /server /server
EXPOSE 8080
CMD ["/server"]
