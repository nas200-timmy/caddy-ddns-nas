#!/bin/sh
# cddns 安装 / 卸载脚本（自包含，不依赖同目录其他文件）
#
# 用法：
#   sudo ./install.sh                    用同目录（或 ../bin）的 cddns 二进制安装
#   sudo ./install.sh /path/to/cddns     用指定二进制安装
#   sudo ./install.sh --version v1.0.0   从 GitHub Releases 下载对应架构发布包并安装
#   sudo ./install.sh --dry-run          只打印将要执行的动作，不落盘
#   sudo ./install.sh --uninstall        卸载（保留配置与数据目录）
#
# 环境变量：
#   CDDNS_BIN        安装目标路径（默认 /usr/local/bin/cddns）
#   CDDNS_CONF       配置目录（默认 /etc/cddns）
#   CDDNS_DATA_DIR   数据目录，证书存储（默认 /var/lib/cddns）
#   CDDNS_SERVICE    systemd 单元路径（默认 /etc/systemd/system/cddns.service）
#   CDDNS_REPO       GitHub 仓库 owner/repo（默认 nas200-timmy/cddns）
#   CDDNS_BASE_URL   自定义下载前缀，优先于 CDDNS_REPO（内网镜像/自建发布源）
set -eu

# 本项目仓库：发布包下载来源，改仓库名或迁移时用 CDDNS_REPO 覆盖。
DEFAULT_REPO=nas200-timmy/cddns

BIN=${CDDNS_BIN:-/usr/local/bin/cddns}
CONF=${CDDNS_CONF:-/etc/cddns}
DATA=${CDDNS_DATA_DIR:-/var/lib/cddns}
SERVICE=${CDDNS_SERVICE:-/etc/systemd/system/cddns.service}
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)

SRC=""
VERSION=""
DRY_RUN=0
UNINSTALL=0
TMPDIR_PKG=""

usage() {
	cat <<'EOF'
cddns 安装脚本

用法:
  install.sh [二进制路径]          安装同目录（或 ../bin）或指定的 cddns
  install.sh --version vX.Y.Z     从 GitHub Releases 下载本机架构发布包并安装
  install.sh --dry-run            只打印将要执行的动作
  install.sh --uninstall          卸载（保留 /etc/cddns 与证书目录）

环境变量: CDDNS_BIN CDDNS_CONF CDDNS_DATA_DIR CDDNS_SERVICE CDDNS_REPO CDDNS_BASE_URL
EOF
}

cleanup() {
	[ -n "$TMPDIR_PKG" ] && rm -rf "$TMPDIR_PKG"
	return 0
}
trap cleanup EXIT

run() {
	if [ "$DRY_RUN" = 1 ]; then
		echo "[dry-run] $*"
	else
		"$@"
	fi
}

# systemd_running 判断本机是否真的由 systemd 启动（容器/WSL 里 systemctl 常存在但不可用）。
systemd_running() {
	[ -d /run/systemd/system ] && command -v systemctl >/dev/null 2>&1
}

while [ $# -gt 0 ]; do
	case "$1" in
	--version)
		[ $# -ge 2 ] || { echo "错误: --version 需要一个版本号，如 v1.0.0" >&2; exit 1; }
		VERSION=$2
		shift 2
		;;
	--dry-run)
		DRY_RUN=1
		shift
		;;
	--uninstall)
		UNINSTALL=1
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	-*)
		echo "错误: 未知参数 $1" >&2
		usage >&2
		exit 1
		;;
	*)
		SRC=$1
		shift
		;;
	esac
done

if [ "$(id -u)" != "0" ] && [ "$DRY_RUN" != 1 ]; then
	echo "请以 root 运行: sudo $0" >&2
	exit 1
fi

if [ "$UNINSTALL" = 1 ]; then
	echo "卸载 cddns ..."
	if systemd_running; then
		run systemctl disable --now cddns || true
	fi
	run rm -f "$SERVICE" "$BIN"
	if systemd_running; then
		run systemctl daemon-reload || true
	fi
	echo ""
	echo "已卸载 $BIN 与 $SERVICE。"
	echo "配置目录 $CONF 与数据目录 $DATA（含凭证与证书）已保留，如需清理请手动删除。"
	exit 0
fi

# 按本机架构下载对应发布包。
download_release() {
	ver=$1
	case "$(uname -m)" in
	x86_64 | amd64) arch=linux-amd64 ;;
	aarch64 | arm64) arch=linux-arm64 ;;
	armv7l | armv6l | arm) arch=linux-armv7 ;;
	*)
		echo "错误: 不支持的架构 $(uname -m)" >&2
		exit 1
		;;
	esac

	if [ -n "${CDDNS_BASE_URL:-}" ]; then
		base=${CDDNS_BASE_URL%/}
	else
		base="https://github.com/${CDDNS_REPO:-$DEFAULT_REPO}/releases/download/${ver}"
	fi

	name="cddns_${ver}_${arch}"
	url="${base}/${name}.tar.gz"
	echo "下载 $url"
	TMPDIR_PKG=$(mktemp -d)
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$url" -o "$TMPDIR_PKG/pkg.tar.gz"
	elif command -v wget >/dev/null 2>&1; then
		wget -qO "$TMPDIR_PKG/pkg.tar.gz" "$url"
	else
		echo "错误: 需要 curl 或 wget 才能下载发布包" >&2
		exit 1
	fi
	tar -xzf "$TMPDIR_PKG/pkg.tar.gz" -C "$TMPDIR_PKG"
	SRC=$(find "$TMPDIR_PKG" -type f -name cddns | head -n 1)
	[ -n "$SRC" ] || {
		echo "错误: 发布包中未找到 cddns 二进制" >&2
		exit 1
	}
	chmod 0755 "$SRC"
}

if [ -n "$VERSION" ]; then
	download_release "$VERSION"
else
	if [ -z "$SRC" ]; then
		if [ -f "$SCRIPT_DIR/cddns" ]; then
			SRC="$SCRIPT_DIR/cddns"
		elif [ -f "$SCRIPT_DIR/../bin/cddns" ]; then
			SRC="$SCRIPT_DIR/../bin/cddns"
		fi
	fi
	# 部分文件系统或下载方式会丢掉可执行位，这里补上
	if [ -n "$SRC" ] && [ -f "$SRC" ] && [ ! -x "$SRC" ]; then
		chmod +x "$SRC" 2>/dev/null || true
	fi
	if [ -z "$SRC" ] || [ ! -x "$SRC" ]; then
		echo "未找到可执行的 cddns 二进制: ${SRC:-$SCRIPT_DIR/cddns}" >&2
		echo "用法: sudo $0 [二进制路径 | --version vX.Y.Z]" >&2
		exit 1
	fi
fi

# systemd 单元：随包附带 scripts/cddns.service 时以它为基准（只重写路径行，
# 避免两份定义漂移）；单独下载本脚本时按实际路径生成。
UNIT_FILE="$SCRIPT_DIR/cddns.service"
if [ -f "$UNIT_FILE" ]; then
	UNIT=$(sed \
		-e "s#^ExecStart=.*#ExecStart=$BIN run --config $CONF/config.yaml#" \
		-e "s#^Environment=CDDNS_CONFIG=.*#Environment=CDDNS_CONFIG=$CONF/config.yaml#" \
		-e "s#^Environment=CDDNS_DATA_DIR=.*#Environment=CDDNS_DATA_DIR=$DATA#" \
		-e "s#^Environment=XDG_DATA_HOME=.*#Environment=XDG_DATA_HOME=$DATA#" \
		-e "s#^Environment=XDG_CONFIG_HOME=.*#Environment=XDG_CONFIG_HOME=$DATA#" \
		"$UNIT_FILE")
else
	UNIT=$(cat <<EOF
[Unit]
Description=cddns - 域名反向代理与免费证书管理
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=$BIN run --config $CONF/config.yaml
Restart=on-failure
RestartSec=5s
# 证书、数据与配置目录（避免依赖 HOME，也避免向任意用户主目录写文件）
Environment=CDDNS_CONFIG=$CONF/config.yaml
Environment=CDDNS_DATA_DIR=$DATA
Environment=XDG_DATA_HOME=$DATA
Environment=XDG_CONFIG_HOME=$DATA

[Install]
WantedBy=multi-user.target
EOF
)
fi

echo "安装 cddns ..."
run mkdir -p "$CONF" "$DATA" "$(dirname "$BIN")"
run install -m 0755 "$SRC" "$BIN"

if [ "$DRY_RUN" = 1 ]; then
	echo "[dry-run] 写入 $SERVICE:"
	printf '%s\n' "$UNIT" | sed 's/^/    /'
else
	printf '%s\n' "$UNIT" >"$SERVICE"
	chmod 0644 "$SERVICE"
fi

if systemd_running; then
	run systemctl daemon-reload
	run systemctl enable cddns || true
	if [ "$DRY_RUN" != 1 ]; then
		if systemctl is-active --quiet cddns; then
			run systemctl restart cddns
		else
			systemctl start cddns 2>/dev/null || echo "提示: 服务启动失败（可能还没有配置），运行向导后重启即可"
		fi
	fi
else
	echo "提示: 未检测到运行中的 systemd，单元文件已写入 $SERVICE，请手动启动:"
	echo "      $BIN run --config $CONF/config.yaml"
fi

echo ""
if [ "$DRY_RUN" = 1 ]; then
	echo "以上为 dry-run 计划，未做任何改动。"
else
	echo "已安装: $BIN"
	echo "配置目录: $CONF（向导生成的 config.yaml 与 Caddyfile 保存在这里）"
	echo "数据目录: $DATA（证书存储）"
	echo ""
	echo "下一步: cddns setup   # 运行配置向导，服务会检测到配置并自动生效"
fi
