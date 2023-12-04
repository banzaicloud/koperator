ARG GO_VERSION=1.19

FROM golang:${GO_VERSION}-alpine AS builder

ENV GOFLAGS="-mod=readonly"

RUN apk add --update --no-cache ca-certificates make git curl mercurial

RUN mkdir -p /build
WORKDIR /build

RUN git clone https://github.com/jamiealquiza/sangrenel.git

WORKDIR /build/sangrenel

RUN go mod download && go build -o /sangrenel

FROM alpine:3.18

RUN apk add --update --no-cache ca-certificates tzdata curl bash

COPY --from=builder /sangrenel /sangrenel

ENTRYPOINT [ "/sangrenel" ]
