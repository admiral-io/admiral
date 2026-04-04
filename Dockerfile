FROM alpine:3.22.1@sha256:4bcff63911fcb4448bd4fdacec207030997caf25e9bea4045fa6c8c44de311d1

ARG USER=admiral
ARG UID=1000
ARG GID=1000

# Create non-root user and group
RUN addgroup -S -g "$GID" "$USER" && \
    adduser -S -u "$UID" -G "$USER" "$USER"

# Minimal upgrade & install only what's necessary
# hadolint ignore=DL3018
RUN apk add --no-cache ca-certificates \
    && apk upgrade --no-cache \
    && update-ca-certificates

# Set working dir and switch to non-root user
WORKDIR /app
USER "$USER"

# Copy application files with ownership
COPY --chown=${USER}:${USER} admiral config.yaml /app/

# Set entrypoint and default command
ENTRYPOINT ["/app/admiral"]
CMD ["start", "--config", "config.yaml"]
