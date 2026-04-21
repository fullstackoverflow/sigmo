FROM alpine:3.21
WORKDIR /app
COPY sigmo /usr/local/bin/sigmo

EXPOSE 9527
ENTRYPOINT ["/usr/local/bin/sigmo"]
CMD ["-config", "/etc/sigmo/config.toml"]
