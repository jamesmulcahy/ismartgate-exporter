FROM golang:alpine AS build-env
RUN apk add git openssh-client
RUN mkdir -p /src
WORKDIR /src
ADD . .
RUN go get 
RUN go build -o goapp

FROM alpine
WORKDIR /app
COPY --from=build-env /src/goapp /app/

WORKDIR /app
EXPOSE 9805
ENTRYPOINT ["/app/goapp"]
