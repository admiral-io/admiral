FROM alpine:3.23.4@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11 AS stage
RUN mkdir -p /app && chown 65532:65532 /app

FROM gcr.io/distroless/static-debian12:nonroot@sha256:a9329520abc449e3b14d5bc3a6ffae065bdde0f02667fa10880c49b35c109fd1

ARG TARGETOS
ARG TARGETARCH

COPY --from=stage --chown=nonroot:nonroot /app /app
COPY --chown=nonroot:nonroot ${TARGETOS}/${TARGETARCH}/admiral-server config.yaml /app/

WORKDIR /app

ENTRYPOINT ["/app/admiral-server"]
CMD ["start", "--config", "config.yaml"]
