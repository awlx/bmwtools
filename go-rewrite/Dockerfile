# Build stage
FROM golang:1.22-alpine AS builder

# Install build dependencies including gcc and required libraries
RUN apk update && apk add --no-cache gcc g++ make musl-dev libc-dev sqlite-dev && \
    gcc --version

WORKDIR /app

# Copy go mod and sum files
COPY go.mod ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o bmwtools-server ./cmd/server/main.go

# Final stage
FROM alpine:latest

# Add runtime dependencies if any CGO-compiled libraries are needed
RUN apk add --no-cache sqlite-libs

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/bmwtools-server .

# Copy static assets and demo data
COPY static/ ./static/
COPY FINAL_DEMO_CHARGING_DATA_SMOOTH_CURVES.JSON ./

# Expose the port the server listens on
EXPOSE 8050

# Run the server
CMD ["./bmwtools-server"]
