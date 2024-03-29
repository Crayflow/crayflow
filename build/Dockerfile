# Build the manager binary
FROM golang:1.17 as builder

WORKDIR /workspace
ENV GOPROXY=https://proxy.golang.com.cn,direct \
    GOPRIVATE=github.com/buhuipao/crayflow \
    GO111MODULE=on
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
#RUN go env && go mod download
COPY vendor/ vendor/

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
#FROM gcr.io/distroless/static:nonroot
#FROM gcr.dockerproxy.com/distroless/static:nonroot
FROM gcr.dockerproxy.com/distroless/static:debug
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
