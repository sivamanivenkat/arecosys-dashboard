FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY *.go index.html styles.css design.js dashboard.js logo.png ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /dashboard . && mkdir -p /out/data/media

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /dashboard /dashboard
COPY --from=build --chown=65532:65532 /out/data /data
ENV ARECOSYS_HOST=0.0.0.0 ARECOSYS_PORT=8765 ARECOSYS_LEAD_FILE=/data/leads.json ARECOSYS_MEDIA_DIR=/data/media
VOLUME /data
EXPOSE 8765
ENTRYPOINT ["/dashboard"]
