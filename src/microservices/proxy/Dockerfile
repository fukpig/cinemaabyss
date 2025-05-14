FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
# Create go.mod and go.sum before building if they don't exist
# (e.g., by running `go mod init cinemaabyss/proxy-service` and `go mod tidy` in src/microservices/proxy)
COPY go.mod ./
RUN go mod download 

# Copy the source code
COPY . .

# Build the application
# Ensure the output binary name matches the CMD instruction
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o proxy-service .

# Use a smaller image for the final application
FROM alpine:latest  

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/proxy-service .

# Expose the application port (should match the PORT env var or default)
EXPOSE 8000

# Command to run the executable
CMD ["./proxy-service"] 