# Build from the repo root (this deployment needs core/ and the service):
#   docker build -f cmd/db-analyzer/Dockerfile -t db-analyzer .
FROM golang:1.27 AS build
WORKDIR /src
ENV GOWORK=off CGO_ENABLED=0
COPY core/go.mod core/go.sum core/
COPY services/mongoanalyzer/go.mod services/mongoanalyzer/go.sum services/mongoanalyzer/
COPY cmd/db-analyzer/go.mod cmd/db-analyzer/go.sum cmd/db-analyzer/
RUN cd cmd/db-analyzer && go mod download
COPY core/ core/
COPY services/mongoanalyzer/ services/mongoanalyzer/
COPY cmd/db-analyzer/ cmd/db-analyzer/
RUN cd cmd/db-analyzer && go build -trimpath -o /out/db-analyzer .

# Analysis scripts run in a real mongosh process, so the runtime image needs
# mongosh (standalone linux-x64 build, which bundles its own OpenSSL).
FROM debian:bookworm-slim AS mongosh
ARG MONGOSH_VERSION=2.12.0
ARG MONGOSH_SHA256=aa42cb826b7b8e655c5481293f5365ddaf1a23e43af07520326e5bbc957838ad
ADD --checksum=sha256:${MONGOSH_SHA256} \
    https://github.com/mongodb-js/mongosh/releases/download/v${MONGOSH_VERSION}/mongosh-${MONGOSH_VERSION}-linux-x64.tgz /tmp/mongosh.tgz
RUN tar -xzf /tmp/mongosh.tgz -C /opt && mv /opt/mongosh-${MONGOSH_VERSION}-linux-x64 /opt/mongosh

FROM debian:bookworm-slim
RUN useradd --system --uid 65532 --home /app app
WORKDIR /app
COPY --from=mongosh /opt/mongosh/bin/mongosh /usr/local/bin/mongosh
COPY --from=build /out/db-analyzer /app/db-analyzer
COPY cmd/db-analyzer/config.yaml /app/config.yaml
# Reports go to /out (mount a volume there); config.yaml's relative
# outputDir only makes sense in the repo.
RUN mkdir /out && chown app /out
ENV OUTPUT_DIR=/out
USER app
# Scripts are embedded in the binary. config.yaml gives the defaults; real
# environment variables (docker run -e / --env-file) override them. There is
# never a .env file in the image.
ENTRYPOINT ["/app/db-analyzer", "-config=/app/config.yaml", "-env-file="]
CMD ["-action", "inspect"]
