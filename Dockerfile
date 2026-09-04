# The runtime stage is scratch because the binary needs nothing beside it: it is
# static, pure Go, and opens no files but the changelog it was pointed at.
FROM golang:1.27-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /action-changelog .

FROM scratch
COPY --from=build /action-changelog /action-changelog
ENTRYPOINT ["/action-changelog"]
