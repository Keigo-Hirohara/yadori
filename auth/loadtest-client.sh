set -eu

kcadm=/opt/keycloak/bin/kcadm.sh

if [ -z "$LOADTEST_CLIENT_SECRET" ]; then
  echo "LOADTEST_CLIENT_SECRET が設定されていません"
  exit 1
fi

$kcadm config credentials \
  --server http://localhost:8080 \
  --realm master \
  --user "$KC_BOOTSTRAP_ADMIN_USERNAME" \
  --password "$KC_BOOTSTRAP_ADMIN_PASSWORD"

existing=$($kcadm get clients -r yadori-booker -q clientId=yadori-loadtest --fields id --format csv --noquotes)
if [ -n "$existing" ]; then
  echo "yadori-loadtest は作成済みです"
  exit 0
fi

$kcadm create clients -r yadori-booker -f - <<EOF
{
  "clientId": "yadori-loadtest",
  "name": "負荷試験",
  "enabled": true,
  "publicClient": false,
  "secret": "$LOADTEST_CLIENT_SECRET",
  "serviceAccountsEnabled": true,
  "standardFlowEnabled": false,
  "directAccessGrantsEnabled": false,
  "protocolMappers": [
    {
      "name": "yadori-api audience",
      "protocol": "openid-connect",
      "protocolMapper": "oidc-audience-mapper",
      "config": {
        "included.custom.audience": "yadori-api",
        "access.token.claim": "true",
        "id.token.claim": "false"
      }
    }
  ]
}
EOF

echo "yadori-loadtest を作成しました"
