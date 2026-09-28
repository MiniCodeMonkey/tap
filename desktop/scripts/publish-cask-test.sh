#!/bin/sh
# Checks publish-cask.sh: no token skips by name, a DMG that was not
# notarized skips by name even with the token (the token exists today, the
# Apple credentials do not), a pre-release skips by name, and with all
# three the cask lands as Casks/tap-desktop.rb in the tap, which here is a
# bare repository on disk reached through HOMEBREW_TAP_URL. The token
# never appears on stdout. Every scenario points at that bare repository
# and git may use only file URLs, so a dropped gate pushes to it (and the
# test names the gate) instead of reaching GitHub; the person's own git
# configuration plays no part.
set -eu

script="$(cd "$(dirname "$0")" && pwd)/publish-cask.sh"
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT
export GIT_ALLOW_PROTOCOL=file GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL="$root/gitconfig"
: > "$GIT_CONFIG_GLOBAL"
printf 'cask "tap-desktop" do\nend\n' > "$root/tap-desktop.rb"

# A tap of the test's own: a bare repository with one commit, so the clone has a branch.
git init -q --bare "$root/tap.git"
git clone -q "$root/tap.git" "$root/seed" 2>/dev/null
( cd "$root/seed" && mkdir Formula && printf 'class Tap < Formula\nend\n' > Formula/tap.rb && git add . && git -c user.name=t -c user.email=t@t commit -q -m seed && git push -q origin HEAD 2>/dev/null )
export HOMEBREW_TAP_URL="$root/tap.git"

out=$(env -u HOMEBREW_TAP_TOKEN TAP_RELEASE_NOTARIZED=yes "$script" 2.1.0 "$root/tap-desktop.rb") || { echo "no token should exit 0"; exit 1; }
[ "$out" = "skipped: cask push (HOMEBREW_TAP_TOKEN is not set)" ] || { echo "wrong skip line: $out"; exit 1; }

out=$(HOMEBREW_TAP_TOKEN=token TAP_RELEASE_NOTARIZED=no "$script" 2.1.0 "$root/tap-desktop.rb") || { echo "not notarized should exit 0"; exit 1; }
[ "$out" = "skipped: cask push (the DMG is not notarized)" ] || { echo "wrong not-notarized line: $out"; exit 1; }
out=$(HOMEBREW_TAP_TOKEN=token env -u TAP_RELEASE_NOTARIZED "$script" 2.1.0 "$root/tap-desktop.rb") || { echo "unknown state should exit 0"; exit 1; }
[ "$out" = "skipped: cask push (the DMG is not notarized)" ] || { echo "an unset state is not notarized: $out"; exit 1; }

out=$(HOMEBREW_TAP_TOKEN=token TAP_RELEASE_NOTARIZED=yes "$script" 2.1.0-beta.1 "$root/tap-desktop.rb") || { echo "a pre-release should exit 0"; exit 1; }
[ "$out" = "skipped: cask push (2.1.0-beta.1 is a pre-release)" ] || { echo "wrong pre-release line: $out"; exit 1; }

[ "$(git --git-dir="$root/tap.git" log --format=%s)" = "seed" ] || { echo "a skipped push reached the tap: $(git --git-dir="$root/tap.git" log --format=%s)"; exit 1; }
if git --git-dir="$root/tap.git" ls-tree -r --name-only HEAD | grep -q '^Casks/'; then echo "a skipped push left a cask in the tap"; exit 1; fi

out=$(HOMEBREW_TAP_TOKEN=token TAP_RELEASE_NOTARIZED=yes "$script" 2.1.0 "$root/tap-desktop.rb") || { echo "the push should succeed: $out"; exit 1; }
echo "$out" | grep -q '^pushed Casks/tap-desktop.rb for 2.1.0 (' || { echo "no pushed line: $out"; exit 1; }
git clone -q "$root/tap.git" "$root/check" 2>/dev/null
[ "$(cat "$root/check/Casks/tap-desktop.rb")" = "$(cat "$root/tap-desktop.rb")" ] || { echo "the cask in the tap differs"; exit 1; }
[ "$(cd "$root/check" && git log -1 --format=%s)" = "Update tap-desktop to 2.1.0" ] || { echo "wrong commit message"; exit 1; }
if echo "$out" | grep -q token; then echo "the token reached stdout"; exit 1; fi

# The same version again changes nothing and says so.
out=$(HOMEBREW_TAP_TOKEN=token TAP_RELEASE_NOTARIZED=yes "$script" 2.1.0 "$root/tap-desktop.rb") || { echo "a repeat should exit 0"; exit 1; }
[ "$out" = "the tap already has this cask for 2.1.0" ] || { echo "wrong repeat line: $out"; exit 1; }

# The release job's route: no HOMEBREW_TAP_URL, so the script clones
# https://github.com/MiniCodeMonkey/homebrew-tap.git, which an insteadOf in
# the test's own global config sends to the bare repository on disk. A git
# on PATH logs every call's argv and the header in its environment, and
# copies the clone's .git/config at the push. The token reaches git only
# through the environment: never in argv, the remote URL, .git/config or
# the global config, and never on stdout or stderr.
token="tap-token-7c41e9"
encoded=$(printf 'x-access-token:%s' "$token" | base64 | tr -d '\n')
printf '[url "file://%s/tap.git"]\n\tinsteadOf = https://github.com/MiniCodeMonkey/homebrew-tap.git\n' "$root" > "$GIT_CONFIG_GLOBAL"
mkdir -p "$root/bin"
real_git=$(command -v git)
cat > "$root/bin/git" <<SHIM
#!/bin/sh
printf 'argv: %s\\n' "\$*" >> "$root/git-argv"
printf '%s|%s|%s\\n' "\${GIT_CONFIG_COUNT:-}" "\${GIT_CONFIG_KEY_0:-}" "\${GIT_CONFIG_VALUE_0:-}" >> "$root/git-env"
[ "\$1" = push ] && cp .git/config "$root/clone-config"
exec "$real_git" "\$@"
SHIM
chmod +x "$root/bin/git"
printf 'cask "tap-desktop" do\n  version "2.2.0"\nend\n' > "$root/tap-desktop-2.2.0.rb"
status=0
out=$(env -u HOMEBREW_TAP_URL PATH="$root/bin:$PATH" HOMEBREW_TAP_TOKEN="$token" TAP_RELEASE_NOTARIZED=yes "$script" 2.2.0 "$root/tap-desktop-2.2.0.rb" 2>"$root/github-err") || status=$?
[ -s "$root/git-argv" ] || { echo "the logging git never ran"; exit 1; }
for leak in "$token" "$encoded"; do
	if grep -Fq "$leak" "$root/git-argv"; then echo "the token reached git's argv"; exit 1; fi
	if [ -f "$root/clone-config" ] && grep -Fq "$leak" "$root/clone-config"; then echo "the token reached the clone's .git/config"; exit 1; fi
	if grep -Fq "$leak" "$GIT_CONFIG_GLOBAL"; then echo "the token reached the global git config"; exit 1; fi
	if printf '%s' "$out" | grep -Fq "$leak" || grep -Fq "$leak" "$root/github-err"; then echo "the token reached the output"; exit 1; fi
done
[ "$status" = 0 ] || { echo "the push down the GitHub route should succeed: $out $(cat "$root/github-err")"; exit 1; }
echo "$out" | grep -q '^pushed Casks/tap-desktop.rb for 2.2.0 (' || { echo "no pushed line down the GitHub route: $out"; exit 1; }
[ "$(git --git-dir="$root/tap.git" log -1 --format=%s)" = "Update tap-desktop to 2.2.0" ] || { echo "the GitHub route did not reach the tap"; exit 1; }
grep -Fq 'url = https://github.com/MiniCodeMonkey/homebrew-tap.git' "$root/clone-config" || { echo "the remote is not the bare GitHub URL: $(grep url "$root/clone-config")"; exit 1; }
while IFS='|' read -r count key value; do
	[ "$count" = 1 ] && [ "$key" = "http.https://github.com/.extraheader" ] || { echo "a git call ran without the header's key: $count $key"; exit 1; }
	[ "$value" = "AUTHORIZATION: basic $encoded" ] || { echo "a git call ran without the token's header"; exit 1; }
	[ "$(printf '%s' "${value#AUTHORIZATION: basic }" | base64 -d)" = "x-access-token:$token" ] || { echo "the header does not decode to the token"; exit 1; }
done < "$root/git-env"

echo "publish-cask.sh is right"
