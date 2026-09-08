FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install git and build tools
RUN apk add --no-cache git gcc musl-dev

# Copy go mod files first for better caching
COPY suppliers/officesolution/go.mod suppliers/officesolution/go.sum ./
RUN go mod download

# Copy source code
COPY suppliers/officesolution/*.go ./

# Build the binary specifically for linux/amd64 with static linking
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64
RUN go build -a -installsuffix cgo -o officesolution-scraper .

# Final stage - minimal runtime image
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /app/officesolution-scraper .

# Ensure execute permissions
RUN chmod +x officesolution-scraper

# Create output directory
RUN mkdir -p /app/output/officesolution

CMD ["./officesolution-scraper"]
