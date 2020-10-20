FROM amazonlinux:latest AS build

RUN yum -y update && rm -rf /var/cache/yum/*
RUN yum install -y  \
      git \
      go

RUN mkdir /aws-signingproxy-admissioncontroller
WORKDIR /aws-signingproxy-admissioncontroller
COPY go.mod .
COPY go.sum .

RUN go env -w GOPROXY=direct
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /go/bin/aws-signingproxy-admissioncontroller

FROM scratch
COPY --from=build /go/bin/aws-signingproxy-admissioncontroller /go/bin/aws-signingproxy-admissioncontroller
ENTRYPOINT ["/go/bin/aws-signingproxy-admissioncontroller"]
