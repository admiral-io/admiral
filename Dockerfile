FROM alpine:3.23.3@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659 AS stage
RUN mkdir -p /app && chown 65532:65532 /app

FROM gcr.io/distroless/static-debian12:nonroot@sha256:a9329520abc449e3b14d5bc3a6ffae065bdde0f02667fa10880c49b35c109fd1

ARG TARGETOS
ARG TARGETARCH

COPY --from=stage --chown=nonroot:nonroot /app /app
COPY --chown=nonroot:nonroot ${TARGETOS}/${TARGETARCH}/admiral-server config.yaml /app/

WORKDIR /app

ENTRYPOINT ["/app/admiral-server"]
CMD ["start", "--config", "config.yaml"]
