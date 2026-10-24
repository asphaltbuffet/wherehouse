# Built by goreleaser (dockers_v2). The build context holds only the
# prebuilt binaries under $TARGETPLATFORM/, so `docker build .` from a
# checkout will not work.
FROM gcr.io/distroless/static-debian13:nonroot

ARG TARGETPLATFORM

# Distroless has no shell, so create a nonroot-owned /data by copying the
# base image's empty /tmp. Named volumes inherit this ownership on first use.
COPY --from=gcr.io/distroless/static-debian13:nonroot --chown=65532:65532 /tmp /data
COPY $TARGETPLATFORM/wherehouse /usr/bin/wherehouse

# /dev/stderr only works while logging.max_size_mb is 0 (rotation off).
# Replace once a real stderr sink lands: github.com/asphaltbuffet/wherehouse/issues/257
ENV WHEREHOUSE_DATABASE_PATH=/data/wherehouse.db \
    WHEREHOUSE_LOGGING_FILE_PATH=/dev/stderr

USER 65532:65532
WORKDIR /data
VOLUME /data
EXPOSE 8080

ENTRYPOINT ["/usr/bin/wherehouse", "serve", "--bind", "0.0.0.0"]
