FROM debian:bookworm-slim
WORKDIR /app
COPY pollara_exporter /app/exporter
EXPOSE 9102
CMD ["/app/exporter"]
