# Build the manager binary
FROM golang:1.12.1 as builder

# Copy in the go src
WORKDIR /go/src/github.com/banzaicloud/kafka-operator
COPY pkg/    pkg/
COPY cmd/    cmd/
COPY internal/ internal/
COPY vendor/ vendor/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager github.com/banzaicloud/kafka-operator/cmd/manager

# Copy the controller-manager into a thin image
FROM ubuntu:latest
WORKDIR /
COPY --from=builder /go/src/github.com/banzaicloud/kafka-operator/manager .
ENTRYPOINT ["/manager"]
