FROM golang:1.21-alpine

WORKDIR /app

# Install git for fetching dependencies
RUN apk add --no-cache git

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY suppliers/officesolution/*.go ./

# Create output directory
RUN mkdir -p /app/output/officesolution

# Build the scraper
RUN go build -o officesolution-scraper . && chmod +x officesolution-scraper

# Default command
CMD ["./officesolution-scraper"]
