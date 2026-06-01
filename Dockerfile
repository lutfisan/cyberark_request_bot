# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o cybarbot ./cmd/cybarbot

# Final stage
FROM scratch

WORKDIR /

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/cybarbot /cybarbot
# Copy CA certificates so HTTPS requests work
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Expose port for webhook mode
EXPOSE 8443

# Command to run the executable
ENTRYPOINT ["/cybarbot"]
