FROM golang:alpine as builder

ENV CGO_ENABLED 0

WORKDIR /
COPY . /

RUN go build -o /assets/in ./container_images/gce-img-resource/cmd/in/main.go
RUN go build -o /assets/check ./container_images/gce-img-resource/cmd/check/main.go

FROM scratch AS resource
COPY --from=builder assets/ /opt/resource/
RUN chmod +x /opt/resource/*
COPY ./container_images/gce-img-resource/Dockerfile .

FROM resource
