FROM golang AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /sweep
FROM alpine AS alpine
FROM scratch
COPY --from=alpine /etc/ssl /etc/ssl
COPY --from=build /sweep /sweep
ENTRYPOINT ["/sweep"]
