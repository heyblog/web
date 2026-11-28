#!/bin/sh
set -eu

# Installed root-owned; the legacy service user invokes this through sudo.
if [ "$(id -u)" -ne 0 ]; then
  exec /usr/bin/sudo -n -- /usr/local/sbin/heyblog-deploy-docker \
    "${API_IMAGE-}" "${WEB_IMAGE-}" "${WORKER_IMAGE-}" "$@"
fi

[ "$#" -ge 4 ] || exit 1
api_image=$1
web_image=$2
worker_image=$3
shift 3

is_hex_digest() {
  [ "${#1}" -eq 64 ] || return 1
  case "$1" in *[!0-9a-f]*) return 1 ;; esac
}

image_allowed() {
  prefix="sgccr.ccs.tencentyun.com/heyblog/$1@sha256:"
  case "$2" in
    '') return 0 ;;
    "$prefix"*) is_hex_digest "${2#"$prefix"}" ;;
    *) return 1 ;;
  esac
}
image_allowed api "$api_image" && image_allowed web "$web_image" &&
  image_allowed worker "$worker_image" || exit 1

case "$1" in
  compose)
    # The Compose manifest, its includes, and the env file are managed by the host administrator.
    # shellcheck source=/dev/null
    . /etc/heyblog/deployer-docker.conf
    [ "${2-}" = --env-file ] && [ "${3-}" = "$ENV_FILE" ] &&
      [ "${4-}" = -f ] && [ "${5-}" = "$COMPOSE_FILE" ] || exit 1
    shift 5
    case "$#" in
      2)
        [ "$*" = 'stop worker' ] || exit 1
        ;;
      4)
        [ "$*" = 'pull api web worker' ] && [ -n "$api_image" ] &&
          [ -n "$web_image" ] && [ -n "$worker_image" ] || exit 1
        ;;
      5)
        case "$*" in
          'ps --status running --quiet api' | 'ps --status running --quiet web' | 'ps --status running --quiet worker') ;;
          *) exit 1 ;;
        esac
        ;;
      10 | 11)
        [ -n "$api_image" ] && [ -n "$web_image" ] || exit 1
        case "$*" in
          'up -d --no-deps --pull never --wait --wait-timeout 100 api web')
            [ -z "$worker_image" ] || exit 1
            ;;
          'up -d --no-deps --pull never --wait --wait-timeout 100 api web worker')
            [ -n "$worker_image" ] || exit 1
            ;;
          *) exit 1 ;;
        esac
        ;;
      *) exit 1 ;;
    esac
    set -- compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" "$@"
    ;;
  container)
    [ "$#" -eq 5 ] && [ "$2" = inspect ] && [ "$3" = --format ] && [ "$4" = '{{.Image}}' ] &&
      is_hex_digest "$5" || exit 1
    ;;
  image)
    [ "$#" -eq 5 ] && [ "$2" = inspect ] && [ "$3" = --format ] &&
      [ "$4" = '{{range .RepoDigests}}{{println .}}{{end}}' ] &&
      [ "${5#sha256:}" != "$5" ] && is_hex_digest "${5#sha256:}" || exit 1
    ;;
  *) exit 1 ;;
esac

exec /usr/bin/env -i PATH=/usr/bin:/bin \
  API_IMAGE="$api_image" WEB_IMAGE="$web_image" WORKER_IMAGE="$worker_image" \
  /usr/bin/docker --host unix:///var/run/docker.sock --config /root/.docker "$@"
