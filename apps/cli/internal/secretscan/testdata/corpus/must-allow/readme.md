# Project Documentation

This is a clean documentation file with no secrets.

## Setup

1. Copy `.env.example` to `.env`
2. Fill in your own credentials
3. Run `go build ./...`

## Architecture

The service authenticates via HMAC-SHA256. Keys are stored in the OS keychain.
