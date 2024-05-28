FROM swift:5.8.1 AS builder

ADD ./wayout-cli .
ADD ./wayout-lib /wayout-lib

RUN swift build -c release -v --disable-sandbox
RUN mkdir -p /usr/local/flogram
RUN mv .build/release/* /usr/local/flogram/.

FROM swift:5.8.1-slim
    
COPY --from=builder /usr/local/flogram /usr/local/flogram

ENTRYPOINT /usr/local/flogram/wayout-cli