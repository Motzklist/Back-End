# Use the official Go image to compile the application
FROM golang:1.25-alpine AS builder
LABEL authors="avner"

# Set the working directory for building
WORKDIR /app

# Copying the source code into the container
COPY go.mod ./

RUN go mod tidy

COPY *.go ./

# Building the Go application (
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o motzklist-api-gateway .

# Creating the final smaller image
FROM alpine:latest
# Expose the port the Go application listens on
EXPOSE 8080
# Set the working directory
WORKDIR /root/
# Copy the built binary from the builder stage
COPY --from=builder /app/motzklist-api-gateway .
# Command to run the executable when the container starts
CMD ["./motzklist-api-gateway"]