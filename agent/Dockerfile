FROM golang:1.20.3-alpine3.17 as builder

RUN mkdir -p /agent
WORKDIR /agent

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY ./ ./

RUN ls -lR .
RUN go build -o /agent

FROM golang:1.20.3-alpine3.17 as runner

COPY --from=builder /agent/agent /usr/bin/agent
COPY --from=builder /agent/config.yaml /config.yaml
COPY --from=builder /agent/testlogs/Linux_2k.log /linux.log

ENTRYPOINT ["/bin/sh", "-c"] 
