#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${ROOT_DIR}/.env"

generate_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 24
    return
  fi
  tr -dc 'A-Za-z0-9' </dev/urandom | head -c 48
}

prompt_value() {
  local prompt="$1"
  local default_value="$2"
  local result=""
  if [[ -n "${default_value}" ]]; then
    read -r -p "${prompt} [${default_value}]: " result
    result="${result:-${default_value}}"
  else
    read -r -p "${prompt}: " result
  fi
  printf '%s' "${result}"
}

prompt_secret_or_generate() {
  local prompt="$1"
  local generated
  generated="$(generate_secret)"
  local result=""
  read -r -p "${prompt}（直接回车自动生成）: " result
  if [[ -z "${result}" ]]; then
    result="${generated}"
  fi
  printf '%s' "${result}"
}

normalize_host() {
  local value="${1:-}"
  value="${value#http://}"
  value="${value#https://}"
  value="${value%%/*}"
  printf '%s' "${value}"
}

if [[ -f "${ENV_FILE}" ]]; then
  overwrite="$(prompt_value ".env 已存在，是否覆盖？输入 yes 覆盖" "no")"
  if [[ "${overwrite}" != "yes" ]]; then
    echo "已取消，不改动现有 .env"
    exit 0
  fi
fi

echo "=== 初始化部署环境变量 ==="

public_host="$(prompt_value "服务器公网 IP 或域名" "")"
public_host="$(normalize_host "${public_host}")"
if [[ -z "${public_host}" ]]; then
  echo "服务器公网 IP 或域名不能为空"
  exit 1
fi
mysql_root_password="$(prompt_secret_or_generate "MySQL Root 密码")"
mysql_database="$(prompt_value "数据库名" "infinite_canvas")"
jwt_key="$(prompt_secret_or_generate "JWT 密钥")"
api_key_encryption_key="$(prompt_secret_or_generate "API Key 加密密钥")"
registration_credits="$(prompt_value "新用户注册赠送积分" "0")"
init_admin_username="$(prompt_value "初始管理员用户名" "admin")"
init_admin_password="$(prompt_secret_or_generate "初始管理员密码")"
init_admin_display_name="$(prompt_value "初始管理员显示名" "系统管理员")"
doc_url="$(prompt_value "文档地址" "https://docs.canvas.best")"

cat >"${ENV_FILE}" <<EOF
MYSQL_ROOT_PASSWORD=${mysql_root_password}
MYSQL_DATABASE=${mysql_database}
JWT_KEY=${jwt_key}
API_KEY_ENCRYPTION_KEY=${api_key_encryption_key}
REGISTRATION_CREDITS=${registration_credits}
INIT_ADMIN_USERNAME=${init_admin_username}
INIT_ADMIN_PASSWORD=${init_admin_password}
INIT_ADMIN_DISPLAY_NAME=${init_admin_display_name}
NEXT_PUBLIC_API_URL=/backend-api
NEXT_PUBLIC_DOC_URL=${doc_url}
EOF

echo
echo "已生成 ${ENV_FILE}"
echo "管理员账号：${init_admin_username}"
echo "管理员密码：${init_admin_password}"
echo
echo "下一步执行："
echo "  docker compose up -d --build"
