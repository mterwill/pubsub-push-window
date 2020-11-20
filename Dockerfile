FROM golang:1.15.5 AS builder

COPY . /src
RUN cd /src && go build -mod=vendor -o /tmp/push-window-test ./server

FROM gcr.io/distroless/base

COPY --from=builder /tmp/push-window-test /usr/bin/

ENTRYPOINT [ "/usr/bin/push-window-test" ]
