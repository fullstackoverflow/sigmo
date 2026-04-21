FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY sigmo /usr/local/bin/sigmo

EXPOSE 9527
ENTRYPOINT ["/usr/local/bin/sigmo"]
CMD ["-config", "/etc/sigmo/config.toml"]
