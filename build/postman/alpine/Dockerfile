FROM golang:1.17-alpine3.15 as builder

RUN addgroup -g 1001 postmanq && \
    adduser -S -u 1001 -G postmanq postmanq && \
    mkdir /app

ARG TARGETPLATFORM
COPY ./bin/${TARGETPLATFORM}/ /app/
COPY ./config.yaml /etc/postman.yaml

RUN chown -R 1001:1001 /app && \
    chown 1001:1001 /etc/postman.yaml && \
    chmod +x /app/*

FROM alpine:3.15
WORKDIR /

COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/local/go/lib/time/zoneinfo.zip /usr/local/go/lib/time/zoneinfo.zip
COPY --from=builder /etc/postman.yaml /etc/postman.yaml
COPY --from=builder /app/postmanq /usr/bin/postmanq
COPY --from=builder /app/pmq-grep /usr/bin/pmq-grep
COPY --from=builder /app/pmq-publish /usr/bin/pmq-publish
COPY --from=builder /app/pmq-report /usr/bin/pmq-report

USER postmanq:postmanq

ENV PORT=1080
EXPOSE $PORT

HEALTHCHECK --interval=5s --timeout=1s --start-period=2s --retries=3 CMD wget -nv -t1 --spider http://localhost:${PORT}/health || exit 1

CMD ["/usr/bin/postmanq", "-f", "/etc/postman.yaml"]
