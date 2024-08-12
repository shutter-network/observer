# syntax=docker/dockerfile:1

FROM golang:1.22-alpine AS builder

# Install build-essential and other necessary packages
RUN apk add --no-cache build-base

# Cache build deps for faster builds
RUN mkdir /gomod
COPY /go.* /gomod/
WORKDIR /gomod
RUN --mount=type=cache,target=/root/.cache go mod download

WORKDIR /app
COPY . .
RUN --mount=type=cache,target=/root/.cache go build -o observer .

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/observer .
COPY --from=builder /app/migrations /root/migrations
ENV MIGRATIONS_PATH=/root/migrations
EXPOSE 4000
EXPOSE 23003
ENTRYPOINT ["./observer"]
CMD ["start", "--rpc-url", "$RPC_URL", "--beacon-api-url", "${BEACON_API_URL}", "--sequencer-contract-address", "$SEQUENCER_CONTRACT_ADDRESS", "--validator-registry-contract-address","$VALIDATOR_REGISTRY_CONTRACT_ADDRESS", "--p2pkey", "$P2P_KEY"]
