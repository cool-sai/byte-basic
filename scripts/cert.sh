#!/usr/bin/env bash
# Let's Encrypt 证书签发/续期（http-01 webroot），在服务器宿主机上以 root 运行。
# 证书写入 compose 卷 golang_letsencrypt，挑战文件写入 golang_tlb-acme（tlb 在 80 端口把
# /.well-known/acme-challenge/ 映射到 /acme）。
#
#   scripts/cert.sh issue <domain> <email> [extra-domain ...]   签一张新证书（cert-name = 第一个域名）
#   scripts/cert.sh renew                                       续期所有快到期的 webroot 证书并 reload tlb
#   scripts/cert.sh list                                        列出证书
#
# 注意：现有 lssl.work / img.lssl.work 是 manual dns-01 签的，renew 会跳过/报错，脚本默认不动它们。
# 转成可自动续期见 docs/news-site.md。
set -euo pipefail

PROJECT="${COMPOSE_PROJECT:-golang}"
LE_VOL="${LE_VOL:-${PROJECT}_letsencrypt}"
ACME_VOL="${ACME_VOL:-${PROJECT}_tlb-acme}"
TLB="${TLB_CONTAINER:-${PROJECT}-tlb-1}"
IMAGE="${CERTBOT_IMAGE:-certbot/certbot:latest}"
read -r -a EXTRA <<<"${CERTBOT_EXTRA:-}"

certbot() {
	docker run --rm \
		-v "${LE_VOL}:/etc/letsencrypt" \
		-v "${ACME_VOL}:/acme" \
		"${IMAGE}" "$@"
}

reload_tlb() {
	docker exec "${TLB}" nginx -t && docker exec "${TLB}" nginx -s reload
}

usage() {
	sed -n '2,12p' "$0" >&2
	exit 2
}

cmd="${1:-}"
case "${cmd}" in
issue)
	[ $# -ge 3 ] || usage
	domain="$2"
	email="$3"
	shift 3
	args=(-d "${domain}")
	for d in "$@"; do args+=(-d "${d}"); done
	certbot certonly --webroot -w /acme --non-interactive --agree-tos \
		-m "${email}" --cert-name "${domain}" "${args[@]}" "${EXTRA[@]}"
	reload_tlb
	echo "ok: /etc/letsencrypt/live/${domain}/ 已就绪；到平台里重新发布 TLB 以启用站点证书"
	;;
renew)
	# 只续期 authenticator = webroot 的证书；manual dns-01 的（如当前 lssl.work）跳过，不改它的续期方式。
	mapfile -t names < <(docker run --rm --entrypoint sh -v "${LE_VOL}:/etc/letsencrypt:ro" "${IMAGE}" -c \
		'grep -l "^authenticator *= *webroot" /etc/letsencrypt/renewal/*.conf 2>/dev/null | sed "s#.*/##; s#\.conf\$##"' || true)
	if [ "${#names[@]}" -eq 0 ]; then
		echo "no webroot certificates to renew"
		exit 0
	fi
	rc=0
	for n in "${names[@]}"; do
		echo "renew ${n}"
		certbot renew --non-interactive --cert-name "${n}" "${EXTRA[@]}" || rc=$?
	done
	reload_tlb || rc=$?
	exit "${rc}"
	;;
list)
	certbot certificates
	;;
*)
	usage
	;;
esac
