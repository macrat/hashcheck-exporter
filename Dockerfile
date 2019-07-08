FROM golang:onbuild as builder

ENV CGO_ENABLED=0
ENV GOOS=linux

COPY . /app
WORKDIR /app

RUN go-wrapper download
RUN go build


FROM alpine

RUN apk add --no-cache ca-certificates

COPY --from=builder /app/app /app/hashcheck-exporter
EXPOSE 9998
VOLUME /app/hashcheck.yml
WORKDIR /app

ENTRYPOINT ["/app/hashcheck-exporter"]
