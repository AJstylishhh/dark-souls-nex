# dark-souls-nex

Private **NEX lab** for **Dark Souls Remastered** on Switch (CFW).

Built on [nextendo-nex](https://github.com/NextendoNetwork/nextendo-nex) (same core as dying-light-nex).

## Constants (from dump + RE)

| Item | Value |
|------|--------|
| Access key | **`fafa948a`** (NEX init ~0x5bb738; next to game server id in struct) |
| Game server ID | **`2c126c00`** (`0x2c126c00`) |
| Host | **`g2c126c00-lp1.s.n.srv.nintendo.net`** |
| NEX mark | `NEX_DS-4_3_1` |

Static analysis is strong (ID matches your SNI). **Live test** is the final proof — if signature fails, report the console log line.

## Quick start (your ICS: 192.168.137.1)

```powershell
cd $env:USERPROFILE\OneDrive\Desktop
git clone https://github.com/AJstylishhh/dark-souls-nex.git
cd dark-souls-nex
git pull
go mod tidy
```

### Certs

```powershell
New-Item -ItemType Directory -Path certs -Force | Out-Null
$openssl = "C:\Program Files\OpenSSL-Win64\bin\openssl.exe"
@"
[req]
default_bits = 2048
prompt = no
default_md = sha256
distinguished_name = dn
x509_extensions = v3_req
[dn]
CN = g2c126c00-lp1.s.n.srv.nintendo.net
O = DS Lab
C = US
[v3_req]
subjectAltName = @alt_names
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
[alt_names]
DNS.1 = g2c126c00-lp1.s.n.srv.nintendo.net
"@ | Set-Content certs\openssl.cnf -Encoding ASCII
& $openssl req -x509 -newkey rsa:2048 -nodes -keyout certs\server.key -out certs\server.crt -days 825 -config certs\openssl.cnf
```

### Switch hosts

See `hosts-snippet.txt` → `atmosphere/hosts/emummc.txt` or `sysmmc.txt`, full power-off reboot.

### Run

```powershell
cd "...\dark-souls-nex"
$env:NEXTENDO_HOST = "192.168.137.1"
$env:ACCESS_KEY = "fafa948a"
$env:AUTH_PORT = "443"
$env:SECURE_PORT = "60010"
$env:CERT_FILE = "certs/server.crt"
$env:KEY_FILE = "certs/server.key"
$env:NEXTENDO_REQUIRE_ACCOUNT = "0"
$env:DS_TRACE = "1"
go run .
```

You should see:

```text
[DS] gameServer=2c126c00 ... accessKey=fafa948a
[DS Auth] listening :443
[DS Secure] listening :60010
```

Then open Dark Souls online (soapstone / summon). Watch for `[DS Auth]` / `[DS Secure]` lines.

If the client rejects crypto, paste the server log — key may need a transform (unlikely but possible).
