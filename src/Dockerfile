# syntax=docker/dockerfile:1

# ---- Build stage ----
FROM golang:1.27 AS build-stage

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /server

# ---- Run stage ----
FROM gcr.io/distroless/base-debian12 AS run-stage

WORKDIR /

COPY --from=build-stage /server /server

EXPOSE 8080

ENTRYPOINT ["/server"]
