FROM golang:1.18-bullseye AS builder

WORKDIR /app

COPY . .

RUN go build .

FROM debian:bullseye

WORKDIR /app

COPY --from=builder /app/cobbleday /app/cobbleday

RUN apt-get update && apt-get upgrade -y && apt-get install -y curl

EXPOSE ${PORT}

CMD [ "/app/cobbleday" ]