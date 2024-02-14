# Start from the latest golang base image
FROM golang:latest

# Add Maintainer Info
LABEL maintainer="Zachary Kent <ztkent@gmail.com>"

# Pull the repository
WORKDIR /app
ARG GIT_USERNAME
ARG GIT_TOKEN
RUN git clone https://${GIT_USERNAME}:${GIT_TOKEN}@github.com/Ztkent/data-crawler

# Install Python, Rust, and OpenSSL
RUN apt-get update && \
    apt-get install -y curl pkg-config libssl-dev python3 python3-pip && \
    curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
# Add Rust to PATH
ENV PATH="/root/.cargo/bin:${PATH}"

# Build the Rust app
WORKDIR /app/data-crawler
RUN cargo build --release
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Move the binary where we need it
RUN cp /app/data-crawler/target/release/data-crawler /app/pkg/data-crawler/

# Install Python dependencies
RUN pip3 install -r /app/pkg/data-processor/requirements.txt --break-system-packages

# Build the Go app
RUN go build -o main .

# Expose port 8080 to the outside world
EXPOSE 8080
# Command to run the executable
CMD ["./main"]