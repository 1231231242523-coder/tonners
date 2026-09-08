FROM --platform=$BUILDPLATFORM golang:1.21-alpine AS builder

WORKDIR /app

# Install git and build tools
RUN apk add --no-cache git gcc musl-dev

# Copy source code
COPY suppliers/officesolution/ ./

# Download dependencies
RUN go mod download

# Build the binary specifically for linux/amd64 with static linking
ARG TARGETOS
ARG TARGETARCH
ENV CGO_ENABLED=0
ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH
RUN go build -a -installsuffix cgo -o officesolution-scraper .

# Final stage - minimal runtime image
FROM --platform=$TARGETPLATFORM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /app/officesolution-scraper .

# Ensure execute permissions
RUN chmod +x officesolution-scraper

# Create output directory
RUN mkdir -p /app/output/officesolution

CMD ["./officesolution-scraper"]
