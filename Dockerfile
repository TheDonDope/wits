FROM golang:1.25-alpine as builder

WORKDIR /app
RUN apk add --no-cache curl make nodejs npm

COPY . ./
RUN make install
RUN make build
RUN > /app/.env

FROM scratch
COPY --from=builder /app/bin/wits wits
COPY --from=builder /app/.env .env

EXPOSE 3000
ENTRYPOINT [ "./wits" ]
