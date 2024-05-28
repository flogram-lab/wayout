
FROM golang:1.22.3-alpine

RUN apk add --update protobuf-dev protobuf git
RUN go install github.com/golang/protobuf/protoc-gen-go@v1.5.4
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2

VOLUME /source
VOLUME /out/go

RUN mkdir -p /out/go

ENTRYPOINT protoc \
  --go_out=/out/go \
  --go-grpc_out=/out/go \
  --go_opt=paths=source_relative \
  --go-grpc_opt=paths=source_relative \
  --proto_path=/source \
  flogram.proto