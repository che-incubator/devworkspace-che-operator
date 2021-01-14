# Build the manager binary
FROM golang:1.13 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build \
  -a -o _output/bin/devworkspace-che-operator \
  -gcflags all=-trimpath=/ \
  -asmflags all=-trimpath=/ \
  main.go

FROM registry.access.redhat.com/ubi8-minimal:8.2-349
WORKDIR /
COPY --from=builder /workspace/_output/bin/devworkspace-che-operator /usr/local/bin/devworkspace-che-operator

ENV USER_UID=1001 \
    USER_NAME=devworkspace-che-operator

COPY build/bin /usr/local/bin
RUN /usr/local/bin/user_setup

USER ${USER_UID}

ENTRYPOINT ["/usr/local/bin/entrypoint"]
CMD /usr/local/bin/devworkspace-che-operator
