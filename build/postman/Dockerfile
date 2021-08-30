FROM golang:1.17-alpine as builder

RUN addgroup -g 1001 postmanq && \
    adduser -S -u 1001 -G postmanq postmanq && \
    mkdir /app

ARG TARGETPLATFORM
COPY ./bin/${TARGETPLATFORM}/ /app/
COPY ./config.yaml /etc/postman.yaml

RUN chown -R 1001:1001 /app && \
    chown 1001:1001 /etc/postman.yaml && \
    chmod +x /app/*

FROM scratch
WORKDIR /

COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/local/go/lib/time/zoneinfo.zip /usr/local/go/lib/time/zoneinfo.zip
COPY --from=builder /etc/postman.yaml /etc/postman.yaml
COPY --from=builder /app/postmanq /postmanq
COPY --from=builder /app/pmq-grep /pmq-grep
COPY --from=builder /app/pmq-publish /pmq-publish
COPY --from=builder /app/pmq-report /pmq-report
COPY --from=builder /app/healthcheck /healthcheck

USER postmanq:postmanq

ENV PORT=1080
EXPOSE $PORT

HEALTHCHECK --interval=5s --timeout=1s --start-period=2s --retries=3 CMD [ "/healthcheck" ]

CMD ["/postmanq", "-f", "/etc/postman.yaml"]
