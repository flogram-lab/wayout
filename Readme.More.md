# Wayout Node Installation

## Requirements

- Visual Studio Code
- Devcontainers extension
- Docker Desktop
- Minimum: 8 GB RAM, 32 GB SSD, 4 CPU

## Connectivity notice

Compose stack exposes TCP/UDP ports of Graylog and its Datanode to host.

Note: you can use remote docker context (server) for **bedrock** and make services use external host for graylog/mongodb, to saves resources. 
VPN is required for encrypting communications.

Multi-server setup is not document at this moment.

## Configuration

Copy `.env.example` as `.env` to create configuration.
 
[Create Telegram App](https://core.telegram.org/api/obtaining_api_id) and saved APP_ tokens given by Telegram to the `.env` file.

 TODO: changeme-host.com ?

## Bedrock

Bedrock starting (must be configured before main)

      $ docker-compose --profile bedrock up -d

First time requirements:

 1. Preflight initialize graylog and provision certificate to the datanode using the web browser UI. The password for Basic Auth was printend in container log.

 2. Add **GELF TCP** source in **System > Inputs** with default port. This allows services to log messages to Graylog, which is important for error and problem tracing.

## Main

TODO: make tls-authority, sharing certs, and running telegram login without gRPC?

 - Compile images from sources (once)

            $ docker-compose --profile main build

 - Authorization in Telegram using __interactive mode__ is required, if `TG_PHONE` has changed, or session is not created yet.

            $ docker-compose run -it flo_tg

 - When authorized, normal (non-interactive) mode is fine.

            $ docker-compose --profile main up -d

## Volumes TODO

### CLI TODO: how to use evans.sh?

# Development

 - Extending is mostly done using gRPC API (protobuf) (or flo_rss HTTP endpoint)

 - Use VS Code and **Devcontainers extension** (command: "Reopen in container" for debug)

## Extending

To programmaticaly receive messages saved in monitored sources, a gRPC client could be written.

 - [lazyr](https://github.com/flogram-lab/lazyr) is an example in Swift.

 - `github.com/flogram-lab/wayout/flo_tg/proto` is to be imported in Go programs.

## Compile gRPC changes

Compiling protobuf files and updating go/swift sources is done inside docker. Simply:

      $ docker-compose --profile protoc up
