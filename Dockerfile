# The runtime stage is scratch because the binary needs nothing beside it: it is
# static and pure Go, and it reads the repository through go-git rather than by
# shelling out, so no git binary is installed and git's dubious-ownership check
# never runs against the workspace the runner checked out as another user.
FROM golang:1.27-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /action-changelog .

FROM scratch
COPY --from=build /action-changelog /action-changelog
ENTRYPOINT ["/action-changelog"]
