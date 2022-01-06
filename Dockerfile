FROM golang:1.17-alpine AS builder

RUN mkdir -p /usr/src/
WORKDIR /usr/src/
COPY go.mod go.sum /usr/src/
RUN go mod download

COPY . /usr/src/

RUN go build -v -o /bin/jambon cmd/jambon/main.go

FROM alpine
COPY --from=builder /bin/jambon /bin/jambon
ENTRYPOINT ["/bin/jambon"]