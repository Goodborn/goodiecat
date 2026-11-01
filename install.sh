#!/bin/sh
# Goodiecat installer: sets up Goodiecat with Docker in ~/goodiecat (or
# $GOODIECAT_DIR) and starts it. Run it again any time to update.
#
#   curl -fsSL https://raw.githubusercontent.com/Goodborn/goodiecat/main/install.sh | sh
#
# Answers can be given up front: GOODIECAT_PORT=8080 GOODIECAT_CITY="Berlin, Germany"
set -eu

REPO_RAW="${GOODIECAT_REPO_RAW:-https://raw.githubusercontent.com/Goodborn/goodiecat/main}"
DIR="${GOODIECAT_DIR:-$HOME/goodiecat}"

say()  { printf '\n\033[1;35m%s\033[0m\n' "$*"; }
info() { printf '  %s\n' "$*"; }
fail() { printf '\n\033[1;31m%s\033[0m\n' "$*" >&2; exit 1; }

# piped into sh, stdin is the script itself; questions go to the terminal
ask() {
    if (: < /dev/tty) 2>/dev/null; then
        printf '%s' "$1" > /dev/tty
        read -r reply < /dev/tty || reply=""
    else
        reply=""
    fi
    printf '%s' "${reply:-$2}"
}

fetch() {
    if command -v curl >/dev/null 2>&1; then curl -fsSL "$1"
    elif command -v wget >/dev/null 2>&1; then wget -qO- "$1"
    else fail "Please install curl or wget first."
    fi
}

# taken by another program, or published by a container (Docker's ports
# don't always show up in ss)
port_in_use() {
    if $SUDO docker ps --format '{{.Ports}}' 2>/dev/null | grep -q ":$1->"; then return 0; fi
    if command -v ss >/dev/null 2>&1; then ss -ltn 2>/dev/null | grep -q "[:.]$1 "
    elif command -v netstat >/dev/null 2>&1; then netstat -ltn 2>/dev/null | grep -q "[:.]$1 "
    else return 1
    fi
}

say "🐱 Goodiecat installer"

# --- Docker ---------------------------------------------------------------
if ! command -v docker >/dev/null 2>&1; then
    info "Goodiecat runs in Docker, which isn't installed yet."
    answer=$(ask "  Install Docker now? [Y/n] " "y")
    case "$answer" in
        [Nn]*) fail "Install Docker (https://docs.docker.com/get-docker/), then run this again." ;;
    esac
    fetch https://get.docker.com | sh
fi
SUDO=""
if ! docker info >/dev/null 2>&1; then
    if command -v sudo >/dev/null 2>&1 && sudo docker info >/dev/null 2>&1; then
        SUDO="sudo"
    else
        fail "Docker is installed but not running, or you don't have access to it. Start Docker and try again."
    fi
fi
$SUDO docker compose version >/dev/null 2>&1 || fail "Docker Compose is missing. Update Docker, then run this again."

# --- files ----------------------------------------------------------------
mkdir -p "$DIR/config" "$DIR/assets"
cd "$DIR"
fetch "$REPO_RAW/examples/docker-compose.yml" > docker-compose.yml

if [ -f .env ]; then
    say "Updating your existing Goodiecat in $DIR"
    # shellcheck disable=SC1091
    . ./.env
    PORT="${GOODIECAT_PORT:-8080}"
else
    say "Setting up Goodiecat in $DIR"
    PORT="${GOODIECAT_PORT:-8080}"
    while port_in_use "$PORT"; do PORT=$((PORT + 1)); done
    PORT=$(ask "  Which port should it use? [$PORT] " "$PORT")

    TOKEN=$(head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n')
    printf 'GOODIECAT_APP_TOKEN=%s\nGOODIECAT_PORT=%s\n' "$TOKEN" "$PORT" > .env
    chmod 600 .env
fi

if [ ! -f config/goodiecat.yml ]; then
    fetch "$REPO_RAW/examples/goodiecat.yml" > config/goodiecat.yml
    CITY="${GOODIECAT_CITY-}"
    [ -n "$CITY" ] || CITY=$(ask "  Your city for the weather, e.g. \"Berlin, Germany\" (Enter to skip): " "")
    if [ -n "$CITY" ]; then
        # swap the commented weather example for the real thing
        sed -i.bak \
            -e 's|^          # WEATHER .*|          - type: weather|' \
            -e '/^          # - type: weather$/d' \
            -e "s|^          #   location: .*|            location: $CITY|" \
            -e 's|^          #   units: metric|            units: metric|' \
            -e 's|^          #   hour-format: 24h|            hour-format: 24h|' \
            config/goodiecat.yml && rm -f config/goodiecat.yml.bak
    fi
fi

# --- start ----------------------------------------------------------------
# Compose prefers the environment over .env, so pass the port we settled on
export GOODIECAT_PORT="$PORT"
say "Starting Goodiecat…"
$SUDO docker compose pull -q
$SUDO docker compose up -d

ADDR=$( (hostname -I 2>/dev/null || true) | awk '{print $1}')
[ -n "$ADDR" ] || ADDR=$(ip route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src") print $(i+1)}')
[ -n "$ADDR" ] || ADDR="this-computer"
TOKEN=$(sed -n 's/^GOODIECAT_APP_TOKEN=//p' .env)

say "✅ Goodiecat is running"
info "Open it in a browser:   http://$ADDR:$PORT"
echo
info "Phone app (Android): https://github.com/Goodborn/goodiecat/releases/latest"
info "  Goodiecat address:  $ADDR:$PORT"
info "  App token:          $TOKEN"
echo
info "Your settings:   $DIR/config/goodiecat.yml"
info "Update later:    run this installer again"
