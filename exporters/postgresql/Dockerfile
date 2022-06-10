FROM debian:bullseye

WORKDIR /app

RUN apt-get update -y && apt-get upgrade -y && apt-get install -y curl tar gzip
RUN curl -LO https://github.com/prometheus-community/postgres_exporter/releases/download/v0.10.1/postgres_exporter-0.10.1.linux-amd64.tar.gz
RUN tar -zxf postgres_exporter-0.10.1.linux-amd64.tar.gz

ENV DATA_SOURCE_NAME postgresql://postgres:postgres@pg:5432/postgres?sslmode=disable

EXPOSE 9187

CMD [ "./postgres_exporter-0.10.1.linux-amd64/postgres_exporter", "--auto-discover-databases" ]

