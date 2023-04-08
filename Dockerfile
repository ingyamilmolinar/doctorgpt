FROM golang:1.20.2-alpine3.17 as builder

WORKDIR /

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./
RUN mkdir ./testlogs
COPY testlogs/* ./testlogs/.

RUN go test
RUN go build -o /doctorgpt

FROM scratch

COPY --from=builder /doctorgpt /usr/bin/doctorgpt

COPY config.yaml ./

ARG LOGFILE
ARG OUTDIR
ARG OPENAI_KEY
ENV DEBUG=true
ENV CONFIGFILE=config.yaml
ENV LOGFILE=$LOGFILE
ENV OUTDIR=$OUTDIR
ENV OPENAI_KEY=$OPENAI_KEY

CMD /doctorgpt --logfile=$LOGFILE --outdir=$OUTDIR --configfile=$CONFIGFILE --debug=$DEBUG
