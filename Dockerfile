# Build the manager binary
FROM golang:1.21 as builder
ARG TARGETARCH
WORKDIR /workspace

# Copy Go mod and sum to indicatie which packages to download
COPY go.mod go.mod
COPY go.sum go.sum

# Download go packages
RUN go mod download

# Copy the go source to build from 
COPY cmd/main.go cmd/main.go
COPY api/ api/
COPY internal/controller/ internal/controller/

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -a -o manager cmd/main.go


# Clean minimal image, just enough to run the binary 
FROM harbor.atro.xyz/chainguard/chainguard/static:latest

WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
