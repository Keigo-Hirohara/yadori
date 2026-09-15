#!/usr/bin/env bash
set -euo pipefail

RUNNER_TOKEN=${1:-}
TUNNEL_TOKEN=${2:-}
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
  echo "== $ENV_DIR/.env を作りました。公開 URL とパスワードを編集してください"
fi

echo "== Cloudflare Tunnel（サーバー全体で共有。yadori 以外のアプリも同じトンネルで公開する）"
if ! command -v cloudflared >/dev/null; then
  sudo mkdir -p --mode=0755 /usr/share/keyrings
  curl -fsSL https://pkg.cloudflare.com/cloudflare-main.gpg | sudo tee /usr/share/keyrings/cloudflare-main.gpg >/dev/null
  echo "deb [signed-by=/usr/share/keyrings/cloudflare-main.gpg] https://pkg.cloudflare.com/cloudflared any main" \
    | sudo tee /etc/apt/sources.list.d/cloudflared.list >/dev/null
  sudo apt-get update -qq
  sudo apt-get install -y -qq cloudflared
fi
if systemctl is-active --quiet cloudflared; then
  echo "== cloudflared は稼働中です"
elif [ -n "$TUNNEL_TOKEN" ]; then
  sudo cloudflared service install "$TUNNEL_TOKEN"
else
  echo "== トンネルのトークンが未指定なので cloudflared の登録は飛ばします（後で: sudo cloudflared service install <トークン>）"
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
  echo "== ランナーは登録済みなので登録は飛ばします"
else
  [ -n "$RUNNER_TOKEN" ] || { echo "ランナー登録トークンが要ります: deploy/setup-server.sh <ランナー登録トークン> [トンネルのトークン]"; exit 1; }
  ./config.sh --unattended --url "$REPO_URL" --token "$RUNNER_TOKEN" --name "$(hostname)" --labels yadori-home --replace
fi
if ! ls /etc/systemd/system/actions.runner.* >/dev/null 2>&1; then
  sudo ./svc.sh install "$USER"
fi
sudo ./svc.sh start
sudo ./svc.sh status | head -5

echo "== 完了。main に push すると自動でデプロイされます"
