FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/control-plane .

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/control-plane /app/control-plane
EXPOSE 9090
ENTRYPOINT ["/app/control-plane"]
