FROM golang:1.23-alpine AS builder
ARG BUILD_VERSION=dev-000000000000Z
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-X infinite-canvas-server/config.BuildVersion=${BUILD_VERSION}" -o /server .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai
COPY --from=builder /server /server
EXPOSE 8080
CMD ["/server"]
