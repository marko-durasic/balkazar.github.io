#!/usr/bin/env bash
# MusicOps / Songkick one-time SSH preparation for Linux.
# No browser launch, account login, publishing, AWS deployment or sudo.
set -Eeuo pipefail
trap 'printf "\nSetup stopped at line %s. Nothing was published to Songkick.\n" "$LINENO" >&2' ERR

die() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

command -v uname >/dev/null 2>&1 || die "uname is unavailable."
[[ "$(uname -s)" == "Linux" ]] || die "This script is for a Linux laptop only."
[[ "$EUID" -ne 0 ]] || die "Run as your normal laptop user, not root/sudo."

for bin in git node npm; do
  command -v "$bin" >/dev/null 2>&1 || die "$bin is missing. Install it first, then rerun this script."
done
node_major="$(node -p "Number(process.versions.node.split('.')[0])")"
[[ "$node_major" =~ ^[0-9]+$ ]] || die "Could not check Node.js version."
(( node_major >= 22 )) || die "Node.js 22+ is required; found $(node --version)."

repo_url="https://github.com/marko-durasic/balkazar.github.io.git"
default_repo="$HOME/src/balkazar.github.io"
if [[ -n "$(printenv MUSICOPS_REPO_DIR 2>/dev/null || true)" ]]; then
  repo_dir="$(printenv MUSICOPS_REPO_DIR)"
else
  repo_dir="$default_repo"
fi
[[ "$repo_dir" = /* ]] || die "MUSICOPS_REPO_DIR must be an absolute path."
branch="musicops-pr53"

printf '\nMusicOps SSH preparation (no Songkick login or publication)\n'
printf 'Repository: %s\n' "$repo_dir"

if [[ -e "$repo_dir" ]]; then
  [[ -d "$repo_dir" ]] || die "Target exists but is not a directory."
  [[ -e "$repo_dir/.git" ]] || die "Target exists but is not a Git repository: $repo_dir"
  top="$(git -C "$repo_dir" rev-parse --show-toplevel)" || die "Cannot inspect repository."
  [[ "$(cd "$top" && pwd -P)" == "$(cd "$repo_dir" && pwd -P)" ]] || die "Target is not the root of this repository."

  origin="$(git -C "$repo_dir" remote get-url origin)" || die "Git remote 'origin' missing."
  case "$origin" in
    https://github.com/marko-durasic/balkazar.github.io|\
    https://github.com/marko-durasic/balkazar.github.io.git|\
    git@github.com:marko-durasic/balkazar.github.io.git|\
    ssh://git@github.com/marko-durasic/balkazar.github.io.git)
      ;;
    *) die "Unexpected origin remote: $origin. Will not switch an unrelated repository." ;;
  esac
  [[ -z "$(git -C "$repo_dir" status --porcelain --untracked-files=normal)" ]] ||
    die "The repo contains local changes/untracked files. Save them safely first; nothing was overwritten."
  printf 'Reusing existing clean checkout.\n'
else
  mkdir -p "$(dirname "$repo_dir")"
  printf 'Cloning Balkazar GitHub repository...\n'
  git clone "$repo_url" "$repo_dir"
fi

printf '\nFetching pull request #53...\n'
git -C "$repo_dir" fetch origin pull/53/head
if git -C "$repo_dir" show-ref --verify --quiet "refs/heads/$branch"; then
  git -C "$repo_dir" switch "$branch"
  git -C "$repo_dir" merge --ff-only FETCH_HEAD ||
    die "Existing $branch diverged from PR #53. Kept local changes; resolve manually."
else
  git -C "$repo_dir" switch -c "$branch" FETCH_HEAD
fi

browser_dir="$repo_dir/tools/gigops/browser"
[[ -f "$browser_dir/package.json" ]] || die "Expected MusicOps browser folder not found on PR #53."

printf '\nInstalling local Playwright dependencies (no browser login)...\n'
cd "$browser_dir"
npm install --no-package-lock
./node_modules/.bin/playwright install chromium

printf '\nRunning local safety tests...\n'
npm test
node --check login-songkick.mjs

printf '\nSUCCESS. MusicOps prepared on laptop.\n'
printf 'Branch: %s\n' "$(git -C "$repo_dir" branch --show-current)"
printf 'Next, in the laptop graphical session (not plain SSH):\n'
printf '  cd "%s"\n' "$browser_dir"
printf '  npm run login:songkick\n'
printf 'This opens Songkick Tourbox. You sign in and complete MFA yourself.\n'
printf 'No Songkick event was created, uploaded, or published.\n'
