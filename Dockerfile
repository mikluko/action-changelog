FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /action-changelog .

FROM scratch
COPY --from=build /action-changelog /action-changelog
ENTRYPOINT ["/action-changelog"]
