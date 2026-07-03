FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod main.go ./
RUN go mod tidy
RUN CGO_ENABLED=0 go build -o /out/insucar-api .

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/insucar-api /app/insucar-api
COPY web /app/web
EXPOSE 8080
ENTRYPOINT ["/app/insucar-api"]
