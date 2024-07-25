
# Installation

      $ docker network create --driver bridge flogram-internal

      $ docker-compose --profile bedrock up -d

Before running Wayout, I had to:

 1. Preflight initialize graylog and provision certificate to the datanode using the web browser UI. The password for Basic Auth was printend in container log.

 2. Add **GELF TCP** source in **System > Inputs** with default port. This allows services to log messages to Graylog, which is important for error and problem tracing.

 - If I need to compile images from sources

      $ docker-compose --profile main build

## Configuration

I copied `.env.example` as `.env` to create configuration.

 - Wayout uses tdlib to act as a console Telegram client (flo_tg)
 
 I've [created my Telegram App](https://core.telegram.org/api/obtaining_api_id) and saved APP_ tokens given by Telegram to the `.env` file.

 TODO: changeme-host.com ?

## Run

TODO: make tls-authority, sharing certs, and running telegram login without gRPC?

 - When I change `TG_PHONE`, authorization in Telegram using interactive mode is required.

            $ docker-compose run -it flo_tg

When it did not ask to enter SMS code, I am ready to go in non-interactive (normal) mode:

            $ docker-compose --profile main up -d

-----

### CLI

TODO: Installing CLI alias to compose run

wayout-cli (service client)

flo_tg service commands

      $ wayout ready

      $ wayout sources
      $ wayout sources --monitored

      $ wayout source --on 3773432
      $ wayout source --stream 3773432
      $ wayout source --off 3773432

flo_rss service commands

      $ wayout rss

      $ wayout rss --url 3773432
      $ wayout rss --add 3773432
      $ wayout rss --rm 3773432


If monitoring was not turned on for a source, new messages are not saved/streamed.

### Connectivity

Basic `docker-compose.yml` only exposes TCP/UDP ports of Graylog and its Datanode to host.

Containers linked to the `flogram-internal` bridged network can reach each other.

### Development

##### Requirements

- Visual Studio Code
- Devcontainers extension

#### Extending

To programmaticaly receive messages saved in monitored sources, a gRPC client could be written.

[flogram-lab/lazyr](https://github.com/flogram-lab/lazyr) is an example in Swift.

 `github.com/flogram-lab/wayout/flo_tg/proto` is to be imported in Go programs.

#### Compile gRPC changes

Compiling protobuf files and updating go/swift sources is done inside docker. Simply:

      $ docker-compose --profile protoc up
