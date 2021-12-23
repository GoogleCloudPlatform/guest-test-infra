FROM golang:alpine

RUN apk add --no-cache git

WORKDIR /

# this needs to be run from root of googlecloudplatform/guest-test-infra
COPY . /
RUN CGO_ENABLED=0 go build -o /publish-job-result ./container_images/concourse-metrics/cmd/publish-job-result/main.go
RUN CGO_ENABLED=0 go build -o /publish-time-since ./container_images/concourse-metrics/cmd/publish-time-since/main.go
RUN apk --update add ca-certificates

RUN chmod +x /publish-job-result

FROM scratch

COPY --from=0 publish-job-result /publish-job-result
COPY --from=0 publish-time-since /publish-time-since
COPY --from=0 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# for debugging
COPY container_images/concourse-metrics/Dockerfile /
