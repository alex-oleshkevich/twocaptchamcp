#!/usr/bin/env bash
set -euo pipefail

repository="${TWOCAP_REPOSITORY:-alex-oleshkevich/twocaptchamcp}"
bin_dir="${TWOCAP_INSTALL_DIR:-$HOME/.local/bin}"
shell_name="${TWOCAP_SHELL:-${SHELL:-bash}}"
shell_name="${shell_name##*/}"

fail() {
	printf 'twocap installation failed: %s\n' "$1" >&2
	exit 1
}

command -v curl >/dev/null 2>&1 || fail "curl is required"
command -v tar >/dev/null 2>&1 || fail "tar is required"

case "$(uname -s)" in
Darwin)
	go_os="darwin"
	;;
Linux)
	go_os="linux"
	;;
*)
	fail "unsupported operating system: $(uname -s)"
	;;
esac

case "$(uname -m)" in
x86_64 | amd64)
	go_arch="amd64"
	;;
aarch64 | arm64)
	go_arch="arm64"
	;;
*)
	fail "unsupported architecture: $(uname -m)"
	;;
esac

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

release_json="$tmp_dir/release.json"
curl -fsSL --retry 3 "https://api.github.com/repos/$repository/releases/latest" -o "$release_json" ||
	fail "could not fetch the latest GitHub release"

find_asset_url() {
	local suffix="$1"

	awk -v suffix="$suffix" '
		/"browser_download_url":/ && index(tolower($0), suffix) {
			sub(/^.*"browser_download_url":[[:space:]]*"/, "")
			sub(/".*$/, "")
			print
			exit
		}
	' "$release_json"
}

archive_url="$(find_asset_url "_${go_os}_${go_arch}.tar.gz")"
[[ -n "$archive_url" ]] || fail "no release archive exists for $go_os/$go_arch"
archive_name="${archive_url##*/}"
archive_path="$tmp_dir/$archive_name"
curl -fsSL --retry 3 "$archive_url" -o "$archive_path" || fail "could not download $archive_name"

checksum_url="$(find_asset_url "_checksums.txt")"
[[ -n "$checksum_url" ]] || fail "the latest release has no checksum file"
checksum_path="$tmp_dir/${checksum_url##*/}"
curl -fsSL --retry 3 "$checksum_url" -o "$checksum_path" || fail "could not download the release checksums"

expected_checksum="$(awk -v file="$archive_name" '$2 == file { print $1; exit }' "$checksum_path")"
[[ -n "$expected_checksum" ]] || fail "no checksum found for $archive_name"

if command -v sha256sum >/dev/null 2>&1; then
	actual_checksum="$(sha256sum "$archive_path" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
	actual_checksum="$(shasum -a 256 "$archive_path" | awk '{print $1}')"
else
	fail "sha256sum or shasum is required to verify the download"
fi

[[ "$actual_checksum" == "$expected_checksum" ]] || fail "checksum verification failed"

tar -xzf "$archive_path" -C "$tmp_dir"
binary_path="$tmp_dir/twocap"
[[ -f "$binary_path" ]] || fail "the release archive does not contain twocap"

completion_shell=""
case "$shell_name" in
bash | zsh | fish | pwsh)
	completion_shell="$shell_name"
	;;
powershell)
	completion_shell="pwsh"
	;;
esac

if [[ -n "$completion_shell" ]] && ! "$binary_path" completion "$completion_shell" >/dev/null 2>&1; then
	fail "the latest release does not support $shell_name shell completion; use a newer release"
fi

mkdir -p "$bin_dir"
install -m 0755 "$binary_path" "$bin_dir/twocap"
binary_path="$bin_dir/twocap"

append_once() {
	local file="$1"
	local line="$2"

	mkdir -p "$(dirname "$file")"
	touch "$file"
	if ! grep -Fqx -- "$line" "$file"; then
		printf '\n%s\n' "$line" >> "$file"
	fi
}

printf -v quoted_binary '%q' "$binary_path"
printf -v quoted_bin_dir '%q' "$bin_dir"
path_line="export PATH=$quoted_bin_dir:\$PATH"

case "$shell_name" in
bash)
	rc_file="$HOME/.bashrc"
	append_once "$rc_file" "$path_line"
	append_once "$rc_file" "source <($quoted_binary completion bash)"
	;;
zsh)
	rc_file="${ZDOTDIR:-$HOME}/.zshrc"
	append_once "$rc_file" "$path_line"
	append_once "$rc_file" "source <($quoted_binary completion zsh)"
	;;
fish)
	config_dir="${XDG_CONFIG_HOME:-$HOME/.config}/fish"
	completion_file="$config_dir/completions/twocap.fish"
	mkdir -p "$(dirname "$completion_file")"
	"$binary_path" completion fish > "$completion_file"
	append_once "$config_dir/config.fish" "fish_add_path \"$bin_dir\""
	;;
pwsh | powershell)
	config_dir="${XDG_CONFIG_HOME:-$HOME/.config}/powershell"
	completion_file="$config_dir/completions/twocap.ps1"
	profile_file="${config_dir}/Microsoft.PowerShell_profile.ps1"
	mkdir -p "$(dirname "$completion_file")"
	"$binary_path" completion pwsh > "$completion_file"
	append_once "$profile_file" ". \"$completion_file\""
	;;
*)
	printf 'Installed twocap to %s, but shell completion is not configured for %s.\n' "$bin_dir" "$shell_name" >&2
	exit 0
	;;
esac

printf 'Installed twocap to %s\n' "$bin_dir/twocap"
printf 'Configured %s shell completion. Start a new shell to load it.\n' "$shell_name"
