#!/bin/sh
# Installs, reports, or removes the launchd agent that runs `ingest all` at
# 07:00 and 13:00. The binary and settings are copied out of the checkout,
# since macOS denies a background job access to ~/Documents. Run by the
# Makefile's ingest-* targets, with .env exported.
set -eu

label=com.yeremi777.mlbb-collector.ingest
app_dir="$HOME/Library/Application Support/mlbb-collector"
binary="$app_dir/ingest"
plist="$HOME/Library/LaunchAgents/$label.plist"
log="$HOME/Library/Logs/mlbb-collector/ingest.log"
target="gui/$(id -u)/$label"

install() {
	mkdir -p "$app_dir" "$(dirname "$plist")" "$(dirname "$log")"
	go build -o "$binary" ./cmd/ingest

	umask 077
	tmp=$(mktemp)
	trap 'rm -f "$tmp"' EXIT
	plutil -create xml1 "$tmp"
	plutil -insert Label -string "$label" "$tmp"
	plutil -insert ProgramArguments -array "$tmp"
	plutil -insert ProgramArguments.0 -string "$binary" "$tmp"
	plutil -insert ProgramArguments.1 -string all "$tmp"
	plutil -insert StartCalendarInterval -json '[{"Hour":7,"Minute":0},{"Hour":13,"Minute":0}]' "$tmp"
	plutil -insert StandardOutPath -string "$log" "$tmp"
	plutil -insert StandardErrorPath -string "$log" "$tmp"
	plutil -insert EnvironmentVariables -dictionary "$tmp"
	for key in DB_HOST DB_PORT DB_NAME DB_USERNAME DB_PASSWORD DB_SSLMODE \
		INGEST_TIMEZONE INGEST_USER_AGENT MOONTON_BASE_URL LIQUIPEDIA_BASE_URL; do
		if value=$(printenv "$key"); then
			plutil -insert "EnvironmentVariables.$key" -string "$value" "$tmp"
		fi
	done
	plutil -lint -s "$tmp"

	unload
	mv "$tmp" "$plist"
	chmod 600 "$plist"
	launchctl bootstrap "gui/$(id -u)" "$plist"
	echo "installed $label: ingest all at 07:00 and 13:00, logging to $log"
}

status() {
	if ! state=$(launchctl print "$target" 2>/dev/null); then
		echo "$label is not installed"
		return 1
	fi
	echo "$state" | grep -E '^\s*(state|program|runs|last exit code) ='
	echo "schedule: $(plutil -extract StartCalendarInterval json -o - "$plist")"
	if [ -f "$log" ]; then
		echo "--- last 20 lines of $log"
		tail -n 20 "$log"
	fi
}

# unload waits until launchd has removed the agent, since bootout returns
# before it is gone and an early bootstrap then fails.
unload() {
	launchctl bootout "$target" 2>/dev/null || true
	tries=0
	while launchctl print "$target" >/dev/null 2>&1; do
		tries=$((tries + 1))
		if [ "$tries" -gt 50 ]; then
			echo "$label is still loaded after 10s" >&2
			exit 1
		fi
		sleep 0.2
	done
}

uninstall() {
	unload
	rm -f "$plist" "$binary"
	echo "removed $label; $log is kept"
}

case "${1:-}" in
install) install ;;
status) status ;;
uninstall) uninstall ;;
*)
	echo "usage: $0 install|status|uninstall" >&2
	exit 2
	;;
esac
