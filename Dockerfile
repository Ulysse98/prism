FROM golang:1.27-alpine AS build

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/prism ./cmd/prism

FROM alpine:3.22

WORKDIR /app

RUN addgroup -S prism && adduser -S prism -G prism

COPY --from=build /out/prism /usr/local/bin/prism

RUN mkdir -p /app/data && chown -R prism:prism /app

USER prism

ENTRYPOINT ["prism"]
CMD ["status"]
