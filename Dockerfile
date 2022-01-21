FROM golang:1.17 AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -mod=readonly -a -ldflags "-w -s" -o ./bin/kubescaler ./cmd/kubescaler/main.go

FROM debian:latest

RUN apt update \
 && apt install -y ca-certificates \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /app

RUN addgroup --gid 1000 go && adduser --uid 1000 -gid 1000 go
COPY --from=build --chown=1000:1000 /app/bin/kubescaler /app
RUN ls -al /app

USER 1000:1000
ENTRYPOINT ["./kubescaler"]