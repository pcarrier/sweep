FROM golang:1.21.1 AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /sweep

FROM scratch
COPY --from=build /sweep /sweep
ENTRYPOINT ["/sweep"]
