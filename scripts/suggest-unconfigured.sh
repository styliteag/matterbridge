#!/usr/bin/env bash
# Scan matterbridge logs for UNCONFIGURED-CHANNEL warnings and print
# a deduplicated, ready-to-paste TOML snippet of missing channels.
#
# Usage:
#   scripts/suggest-unconfigured.sh                     # uses `docker compose logs matterbridge`
#   scripts/suggest-unconfigured.sh <service>           # overrides the compose service name
#   scripts/suggest-unconfigured.sh --file <path>       # parse a log file instead
#   docker compose logs | scripts/suggest-unconfigured.sh --stdin
#
# Optional flags:
#   --gateway <name>   gateway name to place snippets under (default: mygateway)
#   --since <t>        passed through to `docker compose logs --since`

set -euo pipefail

GATEWAY="mygateway"
SINCE=""
MODE="compose"
SERVICE="matterbridge"
FILE=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --gateway) GATEWAY="$2"; shift 2;;
        --since)   SINCE="$2"; shift 2;;
        --file)    MODE="file"; FILE="$2"; shift 2;;
        --stdin)   MODE="stdin"; shift;;
        -h|--help)
            sed -n '2,20p' "$0"; exit 0;;
        *)         SERVICE="$1"; shift;;
    esac
done

fetch_logs() {
    case "$MODE" in
        stdin)   cat ;;
        file)    cat -- "$FILE" ;;
        compose)
            if [[ -n "$SINCE" ]]; then
                docker compose logs --no-color --since "$SINCE" "$SERVICE"
            else
                docker compose logs --no-color "$SERVICE"
            fi
            ;;
    esac
}

# Extract account="..." channel="..." from UNCONFIGURED-CHANNEL lines.
pairs=$(fetch_logs \
    | grep -E 'UNCONFIGURED-CHANNEL' \
    | sed -n 's/.*account="\([^"]*\)" channel="\([^"]*\)".*/\1	\2/p' \
    | sort -u)

if [[ -z "$pairs" ]]; then
    echo "# No UNCONFIGURED-CHANNEL entries found." >&2
    exit 0
fi

echo "# Suggested additions for matterbridge.toml"
echo "# (merge these channels into your existing [[gateway.inout]] blocks"
echo "#  under gateway \"$GATEWAY\", or adjust as needed)."
echo
while IFS=$'\t' read -r account channel; do
    [[ -z "$account" ]] && continue
    cat <<EOF
[[gateway.inout]]
    account = "$account"
    channel = "$channel"

EOF
done <<< "$pairs"
