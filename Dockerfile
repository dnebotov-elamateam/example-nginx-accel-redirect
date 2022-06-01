FROM golang:1.17-alpine AS builder
ENV CGO_ENABLED 0
WORKDIR /app
COPY . .
RUN go build \
    # https://stackoverflow.com/questions/22267189/what-does-the-w-flag-mean-when-passed-in-via-the-ldflags-option-to-the-go-comman
    -ldflags "-s -w -extldflags '-static'" \
    -mod=vendor \
    -o /api-gateway \
    ./cmd/api-gateway

FROM gcr.io/distroless/static
COPY --from=builder --chown=nonroot:nonroot /api-gateway /api-gateway
ENV TZ=Europe/Moscow
EXPOSE 9000 
ENTRYPOINT ["/api-gateway"]
