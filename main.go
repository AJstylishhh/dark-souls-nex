// Dark Souls Remastered (Switch) NEX server on nextendo-nex.
//
// Game server ID : 2c126c00  → hostname g2c126c00-lp1.s.n.srv.nintendo.net
// Access key     : from env ACCESS_KEY (required — extract from main NSO / capture)
// NEX build mark : NEX_DS-4_3_1 (from dumped main)
//
// Auth : AUTH_PORT (default 443)
// Secure : SECURE_PORT (default 60010)
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"os"
	"strconv"
	"strings"
	"time"

	nex "github.com/NextendoNetwork/nextendo-nex"
)

const (
	gameServerID  = "2c126c00"
	nexVersion    = 40000
	securePID     = 2
	sessionKeyLen = 32
)

var (
	accessKey      = strings.TrimSpace(envOr("ACCESS_KEY", ""))
	nextendoHost   = envOr("NEXTENDO_HOST", "192.168.137.1")
	authPort       = envOrInt("AUTH_PORT", 443)
	securePort     = envOrInt("SECURE_PORT", 60010)
	certFile       = envOr("CERT_FILE", "certs/server.crt")
	keyFile        = envOr("KEY_FILE", "certs/server.key")
	securePassword = envOr("NEXTENDO_SECURE_PASSWORD", "securepasswordplz1")
	nextendoSecret = loadNextendoSecret()
	requireAccount = os.Getenv("NEXTENDO_REQUIRE_ACCOUNT") == "1"
	relayPortBase  = envOrInt("NEXTENDO_RELAY_PORT_BASE", 40000)
	relayPortSpan  = envOrInt("NEXTENDO_RELAY_PORT_SPAN", 100)
)

func main() {
	if accessKey == "" || len(accessKey) != 8 {
		fmt.Println("[DS] ERROR: set ACCESS_KEY to the 8-character NEX access key from the game.")
		fmt.Println("[DS] Example:  $env:ACCESS_KEY = \"xxxxxxxx\"")
		fmt.Println("[DS] Hostname is known: g2c126c00-lp1.s.n.srv.nintendo.net (gameServer=2c126c00)")
		fmt.Println("[DS] Key was not in plain strings of main; extract from NSO or a live NEX capture.")
		os.Exit(1)
	}

	os.Setenv("NEXTENDO_OPEN_PARTICIPATION", "1")
	if os.Getenv("NEXTENDO_PROBE_REPOINT") == "" {
		os.Setenv("NEXTENDO_PROBE_REPOINT", "1")
	}
	if os.Getenv("NEXTENDO_RELAY_ALLOW_PRIVATE") == "" {
		os.Setenv("NEXTENDO_RELAY_ALLOW_PRIVATE", "1")
	}

	fmt.Println("[DS] Dark Souls Remastered Switch NEX lab")
	fmt.Printf("[DS] gameServer=%s host=g%s-lp1.s.n.srv.nintendo.net accessKey=%s\n", gameServerID, gameServerID, accessKey)

	settings := nex.NewSwitchSettings(accessKey, nexVersion)
	startNNCS()

	relayHost := envOr("NEXTENDO_RELAY_HOST", nextendoHost)
	nex.SetPairRelay(relayHost, relayPortBase, relayPortSpan)
	nex.SetRelayVolontaires(nil, nil, true)
	fmt.Printf("[DS] pair relay on %s UDP %d-%d\n", relayHost, relayPortBase, relayPortBase+relayPortSpan-1)

	secureURL := nex.NewStationURL("prudp")
	secureURL.Set("address", nextendoHost)
	secureURL.SetInt("port", securePort)
	secureURL.SetInt("CID", 1)
	secureURL.SetInt("PID", securePID)
	secureURL.SetInt("sid", 1)
	secureURL.SetInt("stream", 10)
	secureURL.SetInt("type", 2)

	authEndpoint := nex.NewEndpoint(settings)
	authCfg := &nex.AuthConfig{
		Settings:         settings,
		SecurePID:        securePID,
		SecurePassword:   securePassword,
		SecureStationURL: secureURL,
		ServerName:       "Nextendo-DS",
		SessionKeyLength: sessionKeyLen,
		ResolveUser:      resolveUser,
	}
	authEndpoint.Register(nex.ProtocolTicketGranting, traced(authCfg.Handler()))
	authEndpoint.OnRMC = logRMC("Auth")
	authServer := nex.NewServer(authEndpoint)

	secureEndpoint := nex.NewEndpoint(settings)
	secureEndpoint.SetSecureAccount(securePassword, securePID)

	mm := nex.NewMatchmaking()
	secureEndpoint.Register(nex.ProtocolSecureConnection, traced(withUnsignedPIDs(nex.SecureConnectionHandler())))
	secureEndpoint.Register(nex.ProtocolMatchmakeExtension, traced(extensionWithBrowseURLs(mm.ExtensionHandler())))
	secureEndpoint.Register(nex.ProtocolMatchMaking, traced(mm.MatchMakingHandler()))
	secureEndpoint.Register(nex.ProtocolMatchMakingExt, traced(mm.MatchMakingExtHandler()))
	secureEndpoint.Register(nex.ProtocolNATTraversal, traced(nex.NATTraversalHandler()))
	secureEndpoint.Register(nex.ProtocolRanking, traced(nex.RankingHandler()))
	secureEndpoint.Register(nex.ProtocolUtility, traced(nex.UtilityHandler()))
	secureEndpoint.OnRMC = logRMC("Secure")
	secureEndpoint.OnConnect = func(c *nex.Connection) {
		fmt.Printf("[DS Secure] connected pid=%d id=%d addr=%s\n", c.PID, c.ID, c.RemoteAddr)
	}
	secureEndpoint.OnDisconnect = func(c *nex.Connection) {
		mm.RemovePlayer(c.PID)
	}
	secureServer := nex.NewServer(secureEndpoint)
	secureEndpoint.StartReaper()

	proxyProto := os.Getenv("NEXTENDO_PROXY_PROTOCOL") == "1"
	go func() {
		fmt.Printf("[DS Auth] listening :%d (secure → %s)\n", authPort, secureURL.String())
		var err error
		if proxyProto {
			err = authServer.ListenSecureProxy(authPort, certFile, keyFile)
		} else {
			err = authServer.ListenSecure(authPort, certFile, keyFile)
		}
		if err != nil {
			fmt.Printf("[DS Auth] stopped: %v\n", err)
		}
	}()

	fmt.Printf("[DS Secure] listening :%d  (accessKey=%s gameServer=%s)\n", securePort, accessKey, gameServerID)
	if err := secureServer.ListenSecure(securePort, certFile, keyFile); err != nil {
		fmt.Printf("[DS Secure] stopped: %v\n", err)
	}
}

func resolveUser(username string, extraData []byte) (uint64, []byte, bool) {
	sk := sha256.Sum256([]byte("nextendo-src:" + username))
	sourceKey := sk[:]

	if pid, ok := nextendoPIDFromToken(username); ok {
		fmt.Printf("[Auth] nx2 token → pid=%d\n", pid)
		return pid, sourceKey, true
	}
	if n, err := strconv.ParseUint(username, 10, 64); err == nil && n >= 1800000000 {
		fmt.Printf("[Auth] numeric identity pid=%d\n", n)
		if requireAccount {
			return 0, nil, false
		}
		return n, sourceKey, true
	}
	if requireAccount {
		return 0, nil, false
	}
	pid := anonymousPID(username)
	fmt.Printf("[Auth] guest %q → pid=%d\n", username, pid)
	return pid, sourceKey, true
}

func nextendoPIDFromToken(s string) (uint64, bool) {
	if len(nextendoSecret) == 0 || !strings.HasPrefix(s, "nx2.") {
		return 0, false
	}
	parts := strings.Split(s[len("nx2."):], ".")
	if len(parts) != 2 {
		return 0, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return 0, false
	}
	mac := hmac.New(sha256.New, nextendoSecret)
	mac.Write([]byte("nex:" + string(raw)))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(want), []byte(parts[1])) {
		return 0, false
	}
	f := strings.SplitN(string(raw), ".", 3)
	if len(f) != 3 {
		return 0, false
	}
	pid, err := strconv.ParseUint(f[0], 10, 64)
	if err != nil {
		return 0, false
	}
	if exp, err := strconv.ParseInt(f[2], 10, 64); err != nil || time.Now().Unix() > exp {
		return 0, false
	}
	return pid, true
}

func loadNextendoSecret() []byte {
	if v := os.Getenv("NEXTENDO_SECRET"); v != "" {
		return []byte(v)
	}
	path := envOr("NEXTENDO_SECRET_FILE", "nextendo_secret.key")
	if b, err := os.ReadFile(path); err == nil {
		if dec, derr := hex.DecodeString(strings.TrimSpace(string(b))); derr == nil && len(dec) >= 16 {
			return dec
		}
	}
	return nil
}

func anonymousPID(username string) uint64 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(username))
	return 1800000000 + uint64(h.Sum32()%100000000)
}

const methodBrowseWithHostURLs uint32 = 0x35

type gatheringURLs struct {
	gid  uint32
	urls []*nex.StationURL
}

func (g *gatheringURLs) Levels() []nex.Level {
	return []nex.Level{{
		Save: func(o *nex.StreamOut) {
			o.U32(g.gid)
			nex.WriteList(o, g.urls, func(o *nex.StreamOut, u *nex.StationURL) { o.StationURL(u) })
		},
	}}
}

func extensionWithBrowseURLs(base nex.RMCHandler) nex.RMCHandler {
	return func(conn *nex.Connection, req *nex.RMCMessage) *nex.RMCMessage {
		if req.Method != methodBrowseWithHostURLs {
			return base(conn, req)
		}
		as52 := *req
		as52.Method = 0x34
		resp := base(conn, &as52)
		if resp == nil || resp.IsError {
			return resp
		}
		type entry struct {
			ss   nex.MatchmakeSession
			urls []*nex.StationURL
		}
		var kept []*entry
		in := nex.NewStreamIn(resp.Body, conn.Settings)
		count := in.U32()
		for i := uint32(0); i < count && in.Err() == nil; i++ {
			e := &entry{}
			in.Extract(&e.ss)
			if in.Err() != nil {
				break
			}
			if e.ss.HostPID == conn.PID || e.ss.OwnerPID == conn.PID {
				continue
			}
			host := conn.Endpoint.FindConnectionByPID(e.ss.HostPID)
			if host == nil {
				continue
			}
			e.urls = host.Stations()
			kept = append(kept, e)
		}
		out := nex.NewStreamOut(conn.Settings)
		out.U32(uint32(len(kept)))
		for _, e := range kept {
			out.Add(&e.ss)
		}
		out.U32(uint32(len(kept)))
		for _, e := range kept {
			out.Add(&gatheringURLs{gid: e.ss.ID, urls: e.urls})
			fmt.Printf("[DS] browse(53) gid=%d host=%d urls=%d\n", e.ss.ID, e.ss.HostPID, len(e.urls))
		}
		fmt.Printf("[DS] browse(53) pid=%d -> %d session(s)\n", conn.PID, len(kept))
		return nex.NewRMCSuccess(conn.Settings, nex.ProtocolMatchmakeExtension, methodBrowseWithHostURLs, req.CallID, out.Bytes())
	}
}

func logRMC(tag string) func(*nex.Connection, *nex.RMCMessage) {
	return func(c *nex.Connection, req *nex.RMCMessage) {
		fmt.Printf("[DS %s] pid=%d proto=%#x method=%d call=%d\n", tag, c.PID, req.Protocol, req.Method, req.CallID)
		traceRequest(tag, c, req)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envOrInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
