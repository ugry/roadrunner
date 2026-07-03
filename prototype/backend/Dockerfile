FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod main.go main_test.go ./
RUN go get github.com/aws/aws-sdk-go-v2/config github.com/aws/aws-sdk-go-v2/service/sns && go mod tidy
RUN go vet ./... && go test ./...
RUN CGO_ENABLED=0 go build -o /out/insucar-api .

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/insucar-api /app/insucar-api
COPY web /app/web
EXPOSE 8080
ENTRYPOINT ["/app/insucar-api"]
