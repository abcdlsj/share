FROM golang:1-alpine AS builder
RUN apk add --no-cache build-base
WORKDIR /build
COPY . .
RUN go build -ldflags="-s -w" -trimpath -o /dist/timeprogress .
RUN ldd /dist/timeprogress | tr -s [:blank:] '\n' | grep ^/ | xargs -I % install -D % /dist/%
RUN ln -s ld-musl-x86_64.so.1 /dist/lib/libc.musl-x86_64.so.1

FROM gcr.io/distroless/base
COPY --from=builder /dist /

CMD ["/timeprogress"]
