FROM golang:1.20.3-alpine3.17 as builder

RUN mkdir -p /producer
WORKDIR /producer

COPY producer/go.mod ./
COPY producer/go.sum ./
RUN go mod download

COPY producer/ ./
COPY agent/testlogs/Linux_2k.log ./

RUN go build -o /producer

FROM golang:1.20.3-alpine3.17 as runner

COPY --from=builder /producer/producer /usr/bin/producer
COPY --from=builder /producer/Linux_2k.log /linux.log

ENTRYPOINT ["/bin/sh", "-c"] 
