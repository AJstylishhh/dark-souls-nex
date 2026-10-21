# dark-souls-nex

**Lab / private NEX server** for **Dark Souls Remastered** on Nintendo Switch (CFW).

Based on the same stack as [dying-light-nex](https://github.com/AJstylishhh/dying-light-nex) (`nextendo-nex`).

## Confirmed from your dump + pcap

| Item | Value |
|------|--------|
| Game server host | **`g2c126c00-lp1.s.n.srv.nintendo.net`** |
| Game server id | **`2c126c00`** |
| Stack | **Nintendo NEX / Quazal** (`NEX_DS-4_3_1`, `prudp:/address=`, Matchmake, NatTraversal) |
| Title id (cheats/wiki) | `01004AB00A260000` |
| Access key | **Not yet extracted** as plain text from `main` — set `ACCESS_KEY` when you have it |

This is **not** World War Z (Hydra/protobuf). Same family as Dying Light.

## Honest status

| Works after setup | Needs access key + longer online capture |
|-------------------|------------------------------------------|
| Repo, certs, hosts, auth/secure listen | Correct 8-char `ACCESS_KEY` |
| NNCS NAT responder | Full summon/invasion protocol tuning |
| Guest login (`NEXTENDO_REQUIRE_ACCOUNT=0`) | May need DS-specific matchmake quirks |

**Do not expect full co-op on day one** until the access key matches the client.

## Your lab network

| Device | IP |
|--------|-----|
| PC (ICS) | **192.168.137.1** |
| Switch | 192.168.137.x |

## Setup (same idea as Dying Light)

### 1. Clone

```powershell
cd $env:USERPROFILE\OneDrive\Desktop
git clone https://github.com/AJstylishhh/dark-souls-nex.git
cd dark-souls-nex
go mod tidy
```

### 2. Certs (SNI host = game server name)

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

### 3. Switch hosts

Copy from `hosts-snippet.txt` into `atmosphere/hosts/emummc.txt` or `sysmmc.txt`, reboot full power-off.

### 4. Run (after ACCESS_KEY is set)

```powershell
cd "C:\Users\Asif Ali IBS\OneDrive\Desktop\dark-souls-nex"
$env:NEXTENDO_HOST = "192.168.137.1"
$env:AUTH_PORT = "443"
$env:SECURE_PORT = "60010"
$env:CERT_FILE = "certs/server.crt"
$env:KEY_FILE = "certs/server.key"
$env:NEXTENDO_REQUIRE_ACCOUNT = "0"
$env:ACCESS_KEY = "PASTE_8_CHAR_KEY_HERE"
go run .
```

Allow Python/Go or turn Private firewall off only while testing (same as WWZ/DL).

## Finding ACCESS_KEY

1. Longer Wireshark while **placing a summon sign / getting invaded** (not only menu).
2. Or deeper search in `main` NSO (Dying Light had `0da38dbb` in strings; DS did not show a clear 8-hex key in the first pass).

When the key is wrong, auth/ticket usually fails immediately.

## Difference vs Dying Light

| | Dying Light | Dark Souls Switch |
|--|-------------|-------------------|
| Host | `g27332e00-…` | `g2c126c00-…` |
| Access key | `0da38dbb` known | **TBD** |
| NEX string | SDK-style | `NEX_DS-4_3_1` |
| Gameplay | Lobby/coop | Summon / invade / soapstone |

Matchmaking methods may differ; logs (`DS_TRACE=1`) will show what the client calls after auth works.
