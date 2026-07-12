FROM golang:latest

RUN groupadd -g 1000 go && useradd -d /home/go -g 1000 -m go

USER go:go
WORKDIR /home/go
