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
CMD ["./observer", "start", "--rpc-url", "wss://rpc.gnosischain.com/wss", "--p2pkey", "CAESQMl3XPvSPMdLdbVg0M3/5ZenGuk5+Ve0diP0i3F9WH76Xw3ax7KRmBm8CjOcv8FpfINtlW9NrIpsWjeliuGybew="]
