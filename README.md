# CybArBot

CybArBot is a production-grade, Go-based Telegram chatbot that allows authorised PAM administrators and access reviewers to manage CyberArk Privileged Access Manager (PAM) incoming access requests.

## Setup
1. Copy `.env.example` to `.env` and fill in the required values.
2. Run `make build` to build the binary, or `make docker-build` to build the Docker image.

## Usage
- `make run` to run the bot locally.
- `docker-compose up -d` to run the bot in Docker.

## Configuration
See `.env.example` for a full list of configuration options.
The bot supports `longpoll` and `webhook` modes. In webhook mode, it will listen on `WEBHOOK_LISTEN_ADDR` (default `:8443`).
