FROM golang:1 AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/tube-bot .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/tube-bot /tube-bot
ENTRYPOINT ["/tube-bot"]
