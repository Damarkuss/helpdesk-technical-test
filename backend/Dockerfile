# syntax=docker/dockerfile:1
FROM golang:1.24-alpine AS build
WORKDIR /src

# go.sum is optional in the repo; `go mod tidy` creates/updates it from go.mod.
COPY go.mod go.sum* ./
COPY . .
RUN go mod tidy && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.20
RUN adduser -D -u 10001 app
USER app
COPY --from=build /out/server /usr/local/bin/server
EXPOSE 8080
ENTRYPOINT ["server"]
