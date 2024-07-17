# syntax=docker/dockerfile:1

FROM golang:1.22-alpine AS builder
WORKDIR /app

# Install build-essential and other necessary packages
RUN apk add --no-cache build-base

COPY . .
RUN go build -o observer .

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/observer .
COPY --from=builder /app/migrations /root/migrations
ENV MIGRATIONS_PATH=/root/migrations
EXPOSE 8080
CMD ["./observer", "start", "--rpc-url", "$RPC_URL", "--contract-address", "$CONTRACT_ADDRESS", "--p2pkey", "$P2P_KEY"]
