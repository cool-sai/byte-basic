#!/usr/bin/env bash
# 把 GitHub 上的 daily-news 浅拉到 /srv/daily-news。失败时保留旧内容，不删目录。
# 由 daily-news-hook 容器（projects/daily-news-hook）在收到 GitHub push 时调用；也可手动执行（首次 clone / 补拉）。
set -euo pipefail

REPO="${DAILY_NEWS_REPO:-https://github.com/cool-sai/daily-news.git}"
BRANCH="${DAILY_NEWS_BRANCH:-main}"
DIR="${DAILY_NEWS_DIR:-/srv/daily-news}"
LOCK="${DAILY_NEWS_LOCK:-/run/lock/daily-news-pull.lock}"
TRIES="${DAILY_NEWS_TRIES:-3}"

log() { echo "$(date '+%F %T') $*"; }

mkdir -p "$(dirname "${LOCK}")"
exec 9>"${LOCK}"
if ! flock -n 9; then
	log "another pull is running, skip"
	exit 0
fi

export GIT_TERMINAL_PROMPT=0

retry() {
	local i=1
	until timeout 120 "$@"; do
		if [ "${i}" -ge "${TRIES}" ]; then
			return 1
		fi
		log "retry ${i}/${TRIES}: $*"
		sleep $((i * 10))
		i=$((i + 1))
	done
}

if [ ! -d "${DIR}/.git" ]; then
	tmp="${DIR}.tmp.$$"
	rm -rf "${tmp}"
	if ! retry git clone --depth 1 --branch "${BRANCH}" "${REPO}" "${tmp}"; then
		rm -rf "${tmp}"
		log "clone failed"
		exit 1
	fi
	if [ -e "${DIR}" ]; then
		mv "${DIR}" "${DIR}.bak.$(date +%s)"
	fi
	mv "${tmp}" "${DIR}"
	chmod 755 "${DIR}"
	log "cloned $(git -C "${DIR}" rev-parse --short HEAD)"
	exit 0
fi

old="$(git -C "${DIR}" rev-parse --short HEAD 2>/dev/null || echo none)"
if ! retry git -C "${DIR}" fetch --depth 1 origin "${BRANCH}"; then
	log "fetch failed, keep ${old}"
	exit 1
fi
git -C "${DIR}" reset --hard -q FETCH_HEAD
git -C "${DIR}" clean -fdq
new="$(git -C "${DIR}" rev-parse --short HEAD)"
if [ "${old}" != "${new}" ]; then
	log "updated ${old} -> ${new}"
	# 浅仓库的旧对象回收，避免 .git 越积越大
	git -C "${DIR}" reflog expire --expire=now --all
	git -C "${DIR}" gc --prune=now -q || true
fi
