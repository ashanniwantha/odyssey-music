# Stage 1: Build
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy only dependency files first (better layer caching)
COPY go.mod go.sum ./

# Try falling back to direct download if the proxy resets the connection
ENV GOPROXY=https://proxy.golang.org,direct

RUN go mod download

# Copy the rest of the source
COPY . .

# Build a statically linked binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /api ./cmd/api

# Stage 2: Run
FROM alpine:3.21

# Add ca-certification for HTTPs calls
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the binary and migrations from the builder
COPY --from=builder /api /app/api
COPY migrations/ ./migrations/

# Expose the application port (8080 by default)
EXPOSE 8080

CMD ["./api"]