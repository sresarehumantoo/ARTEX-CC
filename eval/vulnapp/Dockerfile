FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /vulnapp .

FROM alpine:3.21
RUN mkdir -p /app/data && \
    echo "db_host=localhost" > /app/data/config.txt && \
    echo "db_password=s3cret_db_pass" >> /app/data/config.txt && \
    echo "api_key=sk-prod-00000000" >> /app/data/config.txt
COPY --from=build /vulnapp /app/vulnapp
EXPOSE 8083
CMD ["/app/vulnapp"]
