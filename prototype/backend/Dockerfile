FROM alpine:3.20
WORKDIR /app
COPY roadrunner-api /app/roadrunner-api
COPY web /app/web
EXPOSE 8080
ENTRYPOINT ["/app/roadrunner-api"]
