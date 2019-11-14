# syntax = docker/dockerfile:experimental
# Build the manager binary
FROM golang:1.13.3 as builder

WORKDIR /workspace
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    GO111MODULE=on

COPY go.mod go.sum ./
RUN go mod download

# Copy the go source
COPY api/ api/
COPY clients/ clients/
COPY controllers/ controllers/
COPY main.go ./

# Build
RUN --mount=type=cache,target=/root/.cache/ \
    go build -v \
        -o bin/manager \
        main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM docker.io/alpine:3.10
COPY --from=builder /workspace/bin/manager /bin/manager
COPY stack-package .


ENTRYPOINT ["/bin/manager"]
