FROM golang:alpine

RUN apk add --no-cache git

WORKDIR /

# this needs to be run from root of googlecloudplatform/guest-test-infra
COPY . /
RUN CGO_ENABLED=0 go build -o /tagger ./autoversioner/cmd/tagger/main.go
RUN CGO_ENABLED=0 go build -o /versiongenerator ./autoversioner/cmd/versiongenerator/main.go

RUN chmod +x /tagger
RUN chmod +x /versiongenerator

FROM scratch

COPY --from=0 tagger /tagger
COPY --from=0 versiongenerator /versiongenerator

# for debugging
COPY container_images/autoversioner/Dockerfile /
