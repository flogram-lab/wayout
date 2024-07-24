#!/bin/bash

DIR=$(realpath $(dirname "$0"))

docker run --rm --net host -it \
    -v $DIR/tls-authority:/cert \
    ghcr.io/ktr0731/evans:latest \
    --tls --cacert /cert/ca-cert.pem --cert /cert/server-cert.pem --certkey /cert/server-key.pem \
    --servername neone.am --host localhost --port 8920 \
    --reflection \
    repl