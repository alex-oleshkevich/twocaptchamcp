set dotenv-load := true

go_cache := env_var_or_default("GOCACHE", "/tmp/twocaptchamcp-go-cache")
compose_file := "compose.yaml"
binary := "dist/twocap"

default: check

fmt:
    gofmt -w $(find . -name '*.go')

test:
    GOCACHE={{go_cache}} go test ./...

vet:
    GOCACHE={{go_cache}} go vet ./...

build:
    GOCACHE={{go_cache}} CGO_ENABLED=0 go build ./...

race:
    GOCACHE={{go_cache}} go test -race ./... -count=1

package version="dev":
    mkdir -p dist
    GOCACHE={{go_cache}} CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version={{version}}" -o {{binary}} ./cmd/twocap

run *args:
    GOCACHE={{go_cache}} go run ./cmd/twocap {{args}}

mcp:
    GOCACHE={{go_cache}} go run ./cmd/twocap mcp

stdio:
    GOCACHE={{go_cache}} go run ./cmd/twocap stdio

install: package
    mkdir -p ~/.local/bin
    cp {{binary}} ~/.local/bin/twocap

docker-build image=env_var_or_default("TWOCAP_IMAGE", "twocap:local"):
    docker build --tag {{image}} .

install-skill:
    npx skills@latest add . --global --skill twocap --yes

compose-config:
    docker compose --file {{compose_file}} config --quiet

compose-up:
    docker compose --file {{compose_file}} up --detach --build

compose-down:
    docker compose --file {{compose_file}} down

compose-logs:
    docker compose --file {{compose_file}} logs --follow

uncloud-deploy:
    uc deploy --file {{compose_file}} --yes

uncloud-logs:
    uc logs notarobot

check: fmt test vet
