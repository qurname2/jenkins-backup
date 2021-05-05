FROM golang:1.14-alpine3.13 as build

WORKDIR /go/src/github.com/qurname2/jenkins-backup

COPY . .

RUN go build -o /out/jenkins-backup .

FROM alpine:3.13

COPY --from=build /out/jenkins-backup /usr/local/bin/jenkins-backup

ENV KUBE_VERSION=1.16.1

RUN apk add curl=7.76.1-r0 --no-cache && \
    curl -sSOL https://storage.googleapis.com/kubernetes-release/release/v${KUBE_VERSION}/bin/linux/amd64/kubectl && \
    chmod 755 kubectl && \
    mv kubectl /usr/bin

CMD ["/usr/local/bin/jenkins-backup"]
