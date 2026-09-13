#!/usr/bin/env bash
set -euo pipefail

TOKEN=${1:-}
REPO_URL=https://github.com/Keigo-Hirohara/yadori
ENV_DIR=$HOME/.config/yadori
RUNNER_DIR=$HOME/actions-runner

if ! command -v docker >/dev/null; then
  echo "== Docker をインストールします"
  curl -fsSL https://get.docker.com | sudo sh
fi
sudo usermod -aG docker "$USER"

mkdir -p "$ENV_DIR"
if [ ! -f "$ENV_DIR/.env" ]; then
  cp "$(dirname "$0")/.env.example" "$ENV_DIR/.env"
  echo "== $ENV_DIR/.env を作りました。PUBLIC_HOST とパスワードを編集してください"
fi

echo "== GitHub Actions ランナーを用意します"
mkdir -p "$RUNNER_DIR"
cd "$RUNNER_DIR"
if [ ! -f config.sh ]; then
  VERSION=$(curl -fsSL https://api.github.com/repos/actions/runner/releases/latest | grep '"tag_name"' | sed 's/.*"v\([^"]*\)".*/\1/')
  curl -fsSL -o runner.tar.gz "https://github.com/actions/runner/releases/download/v${VERSION}/actions-runner-linux-x64-${VERSION}.tar.gz"
  tar xzf runner.tar.gz
  rm runner.tar.gz
fi
if [ -f .runner ]; then
  echo "== 登録済みなので登録は飛ばします"
else
  [ -n "$TOKEN" ] || { echo "登録トークンが要ります: deploy/setup-server.sh <ランナー登録トークン>"; exit 1; }
  ./config.sh --unattended --url "$REPO_URL" --token "$TOKEN" --name "$(hostname)" --labels yadori-home --replace
fi
if ! systemctl is-enabled "actions.runner.*" >/dev/null 2>&1 && ! ls /etc/systemd/system/actions.runner.* >/dev/null 2>&1; then
  sudo ./svc.sh install "$USER"
fi
sudo ./svc.sh start
sudo ./svc.sh status | head -5

echo "== 完了。main に push すると自動でデプロイされます"
