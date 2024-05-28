FROM golang:1.22.3-alpine AS builder

RUN apk add --update git make

ADD go.mod go.sum source/
ADD proto/go.mod proto/go.sum source/proto/
RUN (cd source && go mod download)

ADD . source
RUN go build -C source -o /usr/local/flogram/flo_tg -v

FROM golang:1.22.3-alpine

COPY --from=builder /usr/local/flogram /usr/local/flogram

ENTRYPOINT /usr/local/flogram/flo_tg