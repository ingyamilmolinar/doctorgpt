FROM golang:1.20.2-alpine3.17

WORKDIR /

#RUN apk update
#RUN apk upgrade
#RUN apk add --upgrade linux-headers
#RUN apk add bpftrace

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./

RUN go test
RUN go build -o /doctorgpt

CMD [ "/doctorgpt" ]
