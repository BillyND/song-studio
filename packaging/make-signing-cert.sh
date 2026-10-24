#!/usr/bin/env bash
# Create, once, the self-signed code signing identity "Que Huong Studio Signing" in the login
# keychain. Every release must be signed with it so Sparkle accepts updates. Keep it: back it up
# from Keychain Access (export as .p12) together with the Sparkle key (account que-huong-studio).
set -euo pipefail
NAME="Que Huong Studio Signing"
if security find-identity -p codesigning | grep -q "\"$NAME\""; then echo "already present"; exit 0; fi
TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT
cat > "$TMP/cert.cnf" <<CNF
[req]
distinguished_name = dn
prompt = no
x509_extensions = ext
[dn]
CN = $NAME
O = Que Huong Studio
[ext]
basicConstraints = critical,CA:false
keyUsage = critical,digitalSignature
extendedKeyUsage = critical,codeSigning
CNF
/usr/bin/openssl req -x509 -newkey rsa:2048 -nodes -keyout "$TMP/key.pem" -out "$TMP/cert.pem" -days 3650 -config "$TMP/cert.cnf" 2>/dev/null
PW="$(/usr/bin/openssl rand -hex 16)"
/usr/bin/openssl pkcs12 -export -inkey "$TMP/key.pem" -in "$TMP/cert.pem" -out "$TMP/id.p12" -passout "pass:$PW" -name "$NAME" 2>/dev/null
security import "$TMP/id.p12" -k "$HOME/Library/Keychains/login.keychain-db" -P "$PW" -T /usr/bin/codesign >/dev/null
echo "created \"$NAME\""
