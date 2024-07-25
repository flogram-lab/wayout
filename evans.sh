#!/bin/bash

DIR=$(realpath $(dirname "$0"))

export $(cat "$DIR/.env" | xargs) 2> /dev/null

docker --context default run \
    --rm --net host -it \
    -v "$DIR/tls-authority:/cert" \
    ghcr.io/ktr0731/evans:latest \
    --tls --cacert /cert/ca-cert.pem --cert /cert/server-cert.pem --certkey /cert/server-key.pem \
    --servername "$EVANS_SERVER_NAME" --host "$EVANS_HOST" --port 8920 \
    --reflection \
    repl