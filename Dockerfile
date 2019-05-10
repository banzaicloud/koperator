# Build the manager binary
ARG GO_VERSION=1.12.1

FROM golang:${GO_VERSION}-alpine as builder

RUN apk add --update --no-cache ca-certificates make git curl mercurial

ARG PACKAGE=github.com/banzaicloud/kafka-operator

RUN mkdir -p /go/src/${PACKAGE}
WORKDIR /go/src/${PACKAGE}
COPY pkg/    pkg/
COPY cmd/    cmd/
COPY internal/ internal/
COPY Makefile Gopkg.* /go/src/${PACKAGE}/
RUN make vendor

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager github.com/banzaicloud/kafka-operator/cmd/manager

# Copy the controller-manager into a thin image
FROM alpine:3.9
RUN apk add --no-cache ca-certificates
WORKDIR /
COPY --from=builder /go/src/github.com/banzaicloud/kafka-operator/manager .
ENTRYPOINT ["/manager"]
