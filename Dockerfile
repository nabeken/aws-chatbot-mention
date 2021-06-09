FROM golang:1.16 as build-image

WORKDIR /go/src
COPY . ./

RUN go build -o ../bin/

FROM public.ecr.aws/lambda/go:1

# copy the binary
copy --from=build-image /go/bin/ /var/task/

# Command can be overwritten by providing a different command
CMD ["aws-chatbot-mention"]
