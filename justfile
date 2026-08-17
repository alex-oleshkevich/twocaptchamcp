set dotenv-load := true

go_cache := env_var_or_default("GOCACHE", "/tmp/twocaptchamcp-go-cache")
compose_file := "compose.yaml"
binary := "dist/twocap"

default: check

fmt:
    gofmt -w $(find . -name '*.go')

format-check:
    #!/usr/bin/env bash
    set -euo pipefail
    files=$(gofmt -l .)
    if [[ -n "$files" ]]; then
        printf 'The following files need formatting:\n%s\n' "$files"
        exit 1
    fi

lint:
    GOCACHE={{go_cache}} golangci-lint run

test:
    env -u TWOCAPTCHA_API_KEY GOCACHE={{go_cache}} go test ./...

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

check: format-check lint

release bump:
    #!/usr/bin/env bash
    set -euo pipefail

    case "{{bump}}" in
        patch|minor|major) ;;
        *)
            printf 'Usage: just release patch|minor|major\n' >&2
            exit 2
            ;;
    esac

    branch=$(git branch --show-current)
    if [[ "$branch" != "main" ]]; then
        printf 'Releases must run from the main branch; current branch is %s.\n' "$branch" >&2
        exit 1
    fi

    if [[ -n "$(git status --short)" ]]; then
        printf 'The worktree must be clean before releasing.\n' >&2
        git status --short >&2
        exit 1
    fi

    if [[ -z "$(git config user.name)" || -z "$(git config user.email)" ]]; then
        printf 'Git user.name and user.email must be configured before releasing.\n' >&2
        exit 1
    fi

    git remote get-url origin >/dev/null

    mapfile -t raw_tags < <(git tag --list 'v*' --sort=v:refname)
    tags=()
    for candidate in "${raw_tags[@]}"; do
        if [[ "$candidate" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
            tags+=("$candidate")
        fi
    done

    previous_tag=''
    if (( ${#tags[@]} > 0 )); then
        last_tag_index=$((${#tags[@]} - 1))
        previous_tag="${tags[$last_tag_index]}"
        previous_version="${previous_tag#v}"
        if [[ ! "$previous_version" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
            printf 'Latest tag %s is not a valid stable semantic version.\n' "$previous_tag" >&2
            exit 1
        fi
        major="${BASH_REMATCH[1]}"
        minor="${BASH_REMATCH[2]}"
        patch="${BASH_REMATCH[3]}"
    else
        major=0
        minor=0
        patch=0
    fi

    case "{{bump}}" in
        patch) patch=$((patch + 1)) ;;
        minor) minor=$((minor + 1)); patch=0 ;;
        major) major=$((major + 1)); minor=0; patch=0 ;;
    esac

    next_tag="v$major.$minor.$patch"
    if git rev-parse --verify --quiet "refs/tags/$next_tag" >/dev/null; then
        printf 'Tag %s already exists locally.\n' "$next_tag" >&2
        exit 1
    fi
    if git ls-remote --exit-code --tags origin "refs/tags/$next_tag" >/dev/null 2>&1; then
        printf 'Tag %s already exists on origin.\n' "$next_tag" >&2
        exit 1
    fi

    if [[ -n "$previous_tag" ]]; then
        release_range="$previous_tag..HEAD"
    else
        release_range=HEAD
    fi
    if (( $(git rev-list --count "$release_range") == 0 )); then
        printf 'There are no commits since the previous release (%s).\n' "${previous_tag:-none}" >&2
        exit 1
    fi

    render_range() {
        local range="$1"
        local commits
        commits=$(git log --no-merges --format='- %s (%h)' "$range")
        if [[ -z "$commits" ]]; then
            commits=$(git log --format='- %s (%h)' "$range")
        fi
        printf '%s\n\n' "$commits"
    }

    release_date=$(date -u +%Y-%m-%d)
    changelog_tmp=$(mktemp)
    trap 'rm -f "$changelog_tmp"' EXIT
    {
        printf '# Changelog\n\n'
        printf '## %s - %s\n\n' "$next_tag" "$release_date"
        render_range "$release_range"

        for ((i=${#tags[@]} - 1; i >= 0; i--)); do
            tag="${tags[$i]}"
            tag_date=$(git log -1 --format=%cs "$tag")
            if (( i == 0 )); then
                tag_range="$tag^"
            else
                tag_range="${tags[$((i - 1))]}..$tag^"
            fi
            printf '## %s - %s\n\n' "$tag" "$tag_date"
            render_range "$tag_range"
        done
    } >"$changelog_tmp"

    printf 'Preparing %s from %s.\n\n' "$next_tag" "${previous_tag:-the beginning of history}"
    printf 'Changes in this release:\n'
    render_range "$release_range"
    printf 'This will replace CHANGELOG.md, commit it, create an annotated tag, and atomically push main and %s to origin.\n' "$next_tag"
    printf 'Continue? [y/N] '
    if ! read -r confirmation; then
        printf '\nRelease cancelled.\n'
        exit 0
    fi
    case "${confirmation,,}" in
        y|yes) ;;
        *)
            printf 'Release cancelled.\n'
            exit 0
            ;;
    esac

    mv "$changelog_tmp" CHANGELOG.md
    git add CHANGELOG.md
    git commit --message="chore(release): $next_tag"
    git tag --annotate "$next_tag" --message="Release $next_tag"
    git push --atomic origin main "$next_tag"
