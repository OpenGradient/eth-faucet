# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Ethereum faucet for distributing ERC-20 tokens (or native ETH) on EVM-compatible networks. Forked from `chainflag/eth-faucet`, customized for OpenGradient's $OPG token on Base Sepolia. Go backend with an embedded Svelte frontend served as a single binary.

## Build & Development Commands

```bash
# Build frontend (required before Go build)
go generate

# Build Go binary
go build -o eth-faucet

# Run locally
./eth-faucet -wallet.provider https://sepolia.base.org -wallet.privkey <HEX_KEY>

# Run all Go tests
go test ./...

# Run tests for a specific package
go test ./internal/chain/
go test ./internal/server/

# Lint Go code
golangci-lint run

# Frontend dev server (from web/ directory)
cd web && yarn dev

# Format frontend code
cd web && yarn prettier --write .

# Docker build and run
make build
make run  # requires PRIVATE_KEY env var
```

## Architecture

**Go backend** (`main.go` → `cmd/server.go` → `internal/`):
- `cmd/server.go` — CLI flag definitions and server bootstrap
- `internal/server/` — HTTP server, routes (`/api/info`, `/api/claim`), rate-limiting middleware (TTL cache by address+IP), hCaptcha middleware
- `internal/chain/` — Transaction building/signing (`TxBuilder` interface), keystore decryption, Wei conversion, address validation. Currently hardcoded to legacy transactions (EIP-1559 returns false)

**Svelte frontend** (`web/`):
- `web/src/Faucet.svelte` — Main UI component with address input, ENS resolution (via Cloudflare provider), and hCaptcha integration
- `web/embed.go` — Uses Go's `//go:embed` to bundle `web/dist/` into the binary
- Built with Vite, styled with Bulma CSS

**Data flow**: User submits address → POST `/api/claim` → rate limit check → captcha verify → `TxBuilder.Transfer()` → signed raw transaction → RPC node

## Key Configuration

Server flags with env var fallbacks (see `cmd/server.go`):
- `-wallet.privkey` / `PRIVATE_KEY` — Funder wallet private key
- `-wallet.keyjson` / `KEYSTORE` — Alternative: keystore file path
- `-wallet.provider` — RPC endpoint (default: `https://sepolia.base.org`)
- `-faucet.tokenaddr` — ERC-20 contract address (empty = native ETH)
- `-faucet.amount` — Tokens per claim (default: 0.1)
- `-faucet.minutes` — Rate limit window in minutes (default: 300)
- `-hcaptcha.sitekey` / `-hcaptcha.secret` — Optional captcha (empty secret disables)

Supported chain IDs are hardcoded in `internal/server/server.go` (`chainIDMap`).

## Deployment

- Multi-stage Dockerfile: Node builds frontend → Go compiles binary → Alpine runtime
- GitHub Actions workflow deploys to AWS ECS (`.github/workflows/aws-deploy.yaml`)
- GoReleaser produces cross-platform binaries (`.goreleaser.yml`)
