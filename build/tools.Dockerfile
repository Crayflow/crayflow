# Build the tools binary
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
COPY cmd/ cmd/
COPY api/ api/
RUN mkdir bin

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o bin/set_var cmd/tools/set_var/main.go
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o bin/load_var cmd/tools/load_var/main.go

#FROM gcr.io/distroless/static:nonroot
FROM busybox:latest
WORKDIR /tools
COPY --from=builder /workspace/bin .
RUN chmod a+x ./*