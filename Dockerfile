ARG GOLANG_VERSION=1.25.6

FROM golang:$GOLANG_VERSION AS builder

ARG TARGETARCH
ARG TARGETOS
ARG HITKEEP_VERSION='snapshot'

LABEL org.opencontainers.image.title="HitKeep" \
    org.opencontainers.image.description="A self-hostable, privacy-first web analytics service in a single Go binary." \
    org.opencontainers.image.url="https://hitkeep.com" \
    org.opencontainers.image.source="https://github.com/pascalebeier/hitkeep.git" \
    org.opencontainers.image.version="${HITKEEP_VERSION}" \
    org.opencontainers.image.authors="Pascale Beier (@PascaleBeier)" \
    org.opencontainers.image.licenses="MIT"

WORKDIR /app

COPY . .

RUN go mod download && go mod verify

RUN mkdir -p /var/lib/hitkeep/data

RUN CGO_ENABLED=1 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-w -s -X 'hitkeep/cmd.Version=$HITKEEP_VERSION'" -o /dist/hitkeep ./cmd/hitkeep/main.go

FROM gcr.io/distroless/cc-debian13:nonroot

COPY --from=builder --chown=nonroot:nonroot /var/lib/hitkeep/data /var/lib/hitkeep/data

COPY --from=builder /dist/hitkeep /usr/local/bin/hitkeep

ENV HITKEEP_DB_PATH="/var/lib/hitkeep/data/hitkeep.db"
ENV HITKEEP_ARCHIVE_PATH="/var/lib/hitkeep/data/archive"
VOLUME /var/lib/hitkeep/data

HEALTHCHECK --start-period=60s --start-interval=3s --interval=10s --timeout=3s --retries=3 \
  CMD ["hitkeep", "-healthcheck"]

EXPOSE 8080 7946

ENTRYPOINT ["hitkeep"]
