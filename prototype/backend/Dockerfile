FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum *.go ./
RUN go mod tidy
RUN go vet ./... && go test ./...
RUN CGO_ENABLED=0 go build -o /out/insucar-api .

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/insucar-api /app/insucar-api
COPY web /app/web
EXPOSE 8080
ENTRYPOINT ["/app/insucar-api"]

# build cache bust: force fresh build with connector.go
