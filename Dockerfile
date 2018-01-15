FROM golang:1.9 as builder
COPY . /go/src/github.com/aphistic/dumbcloudthing/
WORKDIR /go/src/github.com/aphistic/dumbcloudthing/
RUN go get -v ./...
RUN CGO_ENABLED=0 GOOS=linux go build -a

FROM alpine:latest
COPY --from=builder /go/src/github.com/aphistic/dumbcloudthing/dumbcloudthing .
CMD ["/dumbcloudthing"]