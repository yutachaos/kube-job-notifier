FROM golang:1.21.5-alpine as build-env
WORKDIR /go/src/app
COPY . /go/src/app
RUN go build -o ./kube-job-notifier *.go

FROM alpine:3.19
LABEL maintainer="yutachaos <bumplive@gmail.com>"

COPY --from=build-env /go/src/app/kube-job-notifier .

ENTRYPOINT ["./kube-job-notifier"]
