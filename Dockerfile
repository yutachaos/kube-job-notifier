FROM golang:1.17.2-alpine as build-env
WORKDIR /go/src/app
COPY . /go/src/app
RUN go build -o ./kube-job-notifier *.go

FROM alpine:3.15
LABEL maintainer="yutachaos <bumplive@gmail.com>"

COPY --from=build-env /go/src/app/kube-job-notifier .

ENTRYPOINT ["./kube-job-notifier"]
