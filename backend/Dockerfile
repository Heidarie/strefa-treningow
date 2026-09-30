FROM golang:1.27.1-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /server ./cmd/server
FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata wget && adduser -D -u 10001 app && mkdir -p /var/log/strefa && chown app:app /var/log/strefa
WORKDIR /app
COPY --from=build /server /app/server
COPY migrations /app/migrations
USER app
EXPOSE 8080
ENTRYPOINT ["/app/server"]
CMD ["api"]
