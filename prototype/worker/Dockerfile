FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.su[m] *.go ./
RUN go mod tidy
RUN go vet ./... && go build -o /out/worker .

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/worker /app/worker
ENTRYPOINT ["/app/worker"]
