FROM --platform=linux/amd64 golang:1.21-alpine AS builder

WORKDIR /app

# Install git and build tools
RUN apk add --no-cache git gcc musl-dev

# Copy source code
COPY suppliers/officesolution/ ./

# Download dependencies
RUN go mod download

# Build the binary specifically for linux/amd64 with static linking
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64
RUN go build -a -installsuffix cgo -ldflags="-s -w" -o officesolution-scraper .

# Final stage - minimal runtime image
FROM --platform=linux/amd64 alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /app/officesolution-scraper .

# Ensure execute permissions and create output directory
RUN chmod +x officesolution-scraper && mkdir -p /app/output/officesolution

ENTRYPOINT ["./officesolution-scraper"]
