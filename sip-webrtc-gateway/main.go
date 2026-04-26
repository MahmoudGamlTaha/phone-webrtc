// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

// sip-webrtc-gateway bridges SIP calls between a PBX and WebRTC browser clients.
// Supports both inbound (PBX calls in) and outbound (browser dials out) with
// bidirectional audio.
//
// Architecture:
//
//	Browser (WebRTC) <--WebSocket--> Gateway <--SIP/UDP--> PBX
//
// Outbound flow (browser dials SIP extension):
//  1. Browser opens WebSocket, establishes WebRTC PeerConnection (audio sendrecv)
//  2. Browser sends "dial" event with target extension
//  3. Gateway sends SIP INVITE to PBX (with digest auth if challenged)
//  4. PBX answers, RTP flows bidirectionally:
//     - Browser mic → WebRTC track → RTP → PBX
//     - PBX → RTP → WebRTC track → Browser speaker
//
// Inbound flow (SIP phone calls in):
//  1. SIP INVITE arrives from PBX
//  2. Gateway answers, creates audio track for WebRTC peers
//  3. RTP audio from SIP is forwarded to all connected browsers
package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
	"github.com/gorilla/websocket"
	"github.com/pion/sdp/v3"
	"github.com/pion/webrtc/v4"
)

var (
	httpPort = flag.Int("http-port", 8080, "Port for HTTP/WebSocket server")
	publicIP = flag.String("public-ip", "", "Public IP for SDP (auto-detected if empty)")
	//	dbPath   = flag.String("db", "root:@tcp(127.0.0.1:3306)/mini_call_crm?parseTime=true", "MySQL DSN (user:password@tcp(host:port)/dbname?parseTime=true)")
	dbPath = flag.String("db", "mini-crm:ZtRlUn3@p7sbo!0l@tcp(62.171.174.59:3306)/mini_crm?parseTime=true", "MySQL DSN (user:password@tcp(host:port)/dbname?parseTime=true)")

	// SIP client flags (for
	// registering with PBX and making outbound calls)
	sipServerAddr = flag.String("sip-server", "173.199.70.125:5666", "SIP server address (host:port), e.g. 173.199.70.125:5668")
	sipUsername   = flag.String("sip-username", "5000", "SIP extension/username to register as")
	sipPassword   = flag.String("sip-password", "881d93316235d6f7492aeb028ab7b588", "SIP password for digest auth")
	sipDomain     = flag.String("sip-domain", "173.199.70.125", "SIP domain (defaults to sip-server host)")

	// SIP server flags (for receiving inbound calls)
	sipListenPort = flag.Int("sip-listen-port", 5666, "Port to listen for inbound SIP traffic")

	// RTP port range (open these UDP ports on your server firewall)
	rtpPortMin = flag.Int("rtp-port-min", 10000, "Minimum RTP port (inclusive)")
	rtpPortMax = flag.Int("rtp-port-max", 20000, "Maximum RTP port (inclusive)")

	// TLS flags for HTTPS (required for browser mic access)
	enableTLS = flag.Bool("tls", false, "Enable HTTPS with auto-generated self-signed cert (required for browser mic access)")
	tlsCert   = flag.String("tls-cert", "", "TLS certificate file (PEM). Overrides auto-generated cert.")
	tlsKey    = flag.String("tls-key", "", "TLS private key file (PEM). Overrides auto-generated key.")

	// Parsed from sipServerAddr at startup
	sipServerPort int

	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	contentTypeHeaderSDP = sip.ContentTypeHeader("application/sdp")
)

// sipCall represents an active SIP call with its RTP connections
type sipCall struct {
	rtpConn    *net.UDPConn                // Local RTP listener
	remoteAddr *net.UDPAddr                // Remote RTP address (latched from actual incoming packets)
	sdpAddr    *net.UDPAddr                // Remote RTP address from SDP (initial target for keepalive)
	audioTrack *webrtc.TrackLocalStaticRTP // Track for SIP→WebRTC direction
	callID     string
	from       string
	to         string
	isOutbound bool
	cancelFunc context.CancelFunc
	latched    bool // True after first RTP packet received (remoteAddr is now latched)
	ringing    bool // True after 180 Ringing received

	// SIP dialog fields for proper BYE construction
	fromTag    string  // From tag from our INVITE
	toTag      string  // To tag from 200 OK response
	cseqNo     uint32  // CSeq number from INVITE
	contactURI sip.Uri // Contact URI from 200 OK response (for BYE Request-URI)
	agentExt   string  // Agent extension used for From header
	agentPass  string  // Agent SIP password for digest auth on BYE

	// CRM fields
	callLogID  int64  // DB call log ID for updating status
	agentID    int64  // Agent who made/received the call
	customerID *int64 // Customer being called (optional)
	startedAt  time.Time
}

// peerState holds per-browser-connection state
type peerState struct {
	ws                 *threadSafeWriter
	pc                 *webrtc.PeerConnection
	call               *sipCall
	callReady          chan struct{}               // Signaled when a SIP call is associated with this peer (replaces busy-wait)
	negotiateMu        sync.Mutex                  // Prevents concurrent renegotiation
	negotiating        bool                        // True when an offer/answer exchange is in progress
	pendingRenegotiate bool                        // True if renegotiation was requested while already negotiating
	placeholderTrack   *webrtc.TrackLocalStaticRTP // Placeholder track to swap back on hangup (avoids renegotiation)
	dialCancel         context.CancelFunc          // Cancel an in-progress dial (before call object exists)
	dialing            bool                        // True when handleDial is running (INVITE in progress)

	// CRM fields
	agentID      int64  // Logged-in agent ID
	agentExt     string // Agent's SIP extension
	agentSIPPass string // Agent's SIP password
	token        string // Auth token
}

// gateway holds all state for the SIP-to-WebRTC bridge
type gateway struct {
	mu    sync.RWMutex
	peers map[*threadSafeWriter]*peerState
	calls map[string]*sipCall // callID -> sipCall

	// SIP (shared UA for client + server so both use same port)
	sipUA     *sipgo.UserAgent
	sipClient *sipgo.Client
	sipServer *sipgo.Server
}

func newGateway() *gateway {
	return &gateway{
		peers: make(map[*threadSafeWriter]*peerState),
		calls: make(map[string]*sipCall),
	}
}

type wsMessage struct {
	Event string `json:"event"`
	Data  string `json:"data"`
}

func main() {
	flag.Parse()

	// Auto-detect public IP if not provided
	if *publicIP == "" {
		// Try external service first (works on cloud servers with NAT)
		resp, err := http.Get("https://api.ipify.org")
		if err == nil {
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err == nil && len(body) > 0 {
				*publicIP = strings.TrimSpace(string(body))
				log.Printf("Auto-detected public IP via ipify: %s", *publicIP)
			}
		}
	}
	if *publicIP == "" {
		// Fallback: use local interface addresses
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			log.Fatalf("Failed to get interface addresses: %v", err)
		}
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					*publicIP = ipnet.IP.String()
					break
				}
			}
		}
		if *publicIP == "" {
			log.Fatal("Could not auto-detect public IP, please specify -public-ip")
		}
	}
	// Warn if detected IP is private (won't work across NAT)
	ip := net.ParseIP(*publicIP)
	if ip != nil && (ip.IsPrivate() || ip.IsLinkLocalUnicast()) {
		log.Printf("WARNING: Detected IP %s is a PRIVATE address. On cloud servers, you MUST specify -public-ip with your server's public IP!", *publicIP)
	}

	// Default SIP domain to server host
	if *sipServerAddr != "" {
		parts := strings.SplitN(*sipServerAddr, ":", 2)
		if *sipDomain == "" {
			*sipDomain = parts[0]
		}
		if len(parts) == 2 {
			if p, err := strconv.Atoi(parts[1]); err == nil {
				sipServerPort = p
			}
		}
	}
	if sipServerPort == 0 {
		sipServerPort = 5060
	}

	log.Printf("Public IP: %s", *publicIP)
	log.Printf("HTTP/WebSocket port: %d", *httpPort)
	log.Printf("SIP server: %s:%d", *sipDomain, sipServerPort)
	log.Printf("RTP port range: %d-%d (open these UDP ports on firewall!)", *rtpPortMin, *rtpPortMax)

	// Initialize database
	if err := initDB(*dbPath); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	gw := newGateway()

	// Create a shared SIP UA (client + server use same socket/port)
	var err error
	gw.sipUA, err = sipgo.NewUA(
		sipgo.WithUserAgent("Pion-SIP-Gateway"),
		sipgo.WithUserAgentTransactionLayerOptions(
			sip.WithTransactionLayerUnhandledResponseHandler(func(res *sip.Response) {
				if res.StatusCode == 200 {
					log.Printf("Retransmitted 200 OK received, re-sending ACK")
					callID := res.CallID()
					gw.mu.RLock()
					call, ok := gw.calls[callID.String()]
					gw.mu.RUnlock()
					if !ok || !call.isOutbound {
						return
					}
					contactURI := sip.Uri{Host: *sipDomain, Port: sipServerPort}
					if contact := res.Contact(); contact != nil {
						contactURI = contact.Address
					}
					ackReq := sip.NewRequest(sip.ACK, contactURI)
					ackReq.AppendHeader(res.Via())
					ackReq.AppendHeader(res.From())
					ackReq.AppendHeader(res.To())
					ackReq.AppendHeader(res.CallID())
					cseq := res.CSeq()
					ackReq.AppendHeader(sip.NewHeader("CSeq", fmt.Sprintf("%d ACK", cseq.SeqNo)))
					ackReq.AppendHeader(sip.NewHeader("Max-Forwards", "70"))
					if err := gw.sipClient.WriteRequest(ackReq); err != nil {
						log.Printf("Failed to re-send ACK: %v", err)
					}
				}
			}),
		),
	)
	if err != nil {
		log.Fatalf("Failed to create SIP UA: %v", err)
	}

	// Create SIP server from shared UA
	gw.sipServer, err = sipgo.NewServer(gw.sipUA)
	if err != nil {
		log.Fatalf("Failed to create SIP Server: %v", err)
	}

	// Register SIP server handlers
	gw.sipServer.OnInvite(gw.onSIPInvite)
	gw.sipServer.OnBye(gw.onSIPBye)
	gw.sipServer.OnAck(gw.onSIPAck)
	gw.sipServer.OnOptions(func(req *sip.Request, tx sip.ServerTransaction) {
		tx.Respond(sip.NewResponseFromRequest(req, 200, "OK", nil))
	})

	// Start SIP server listener (blocks until ready)
	go func() {
		log.Printf("Starting SIP listener on UDP :%d", *sipListenPort)
		if err := gw.sipServer.ListenAndServe(context.TODO(), "udp", fmt.Sprintf("0.0.0.0:%d", *sipListenPort)); err != nil {
			log.Fatalf("SIP server failed: %v", err)
		}
	}()

	// Create SIP client from shared UA (sends from same port as server)
	// Must use public IP so PBX can route responses (200 OK) back to us
	gw.sipClient, err = sipgo.NewClient(gw.sipUA,
		sipgo.WithClientAddr(fmt.Sprintf("%s:%d", *publicIP, *sipListenPort)),
		sipgo.WithClientHostname(*publicIP),
	)
	if err != nil {
		log.Fatalf("Failed to create SIP Client: %v", err)
	}
	log.Printf("SIP client using address: %s:%d", *publicIP, *sipListenPort)

	// Start SIP client registration (if configured)
	if *sipServerAddr != "" && *sipUsername != "" {
		go gw.startSIPRegistration()
	} else {
		log.Printf("SIP client not configured (need -sip-server, -sip-username, -sip-password). Only inbound calls will work.")
	}

	// Start HTTP/WebSocket server (blocks)
	gw.startHTTPServer()
}

// startSIPRegistration runs a loop to register with the PBX
func (gw *gateway) startSIPRegistration() {
	for {
		if err := gw.registerSIP(); err != nil {
			log.Printf("SIP registration failed: %v, retrying in 30s...", err)
			time.Sleep(30 * time.Second)
			continue
		}
		log.Printf("SIP registered as %s@%s", *sipUsername, *sipDomain)

		// Re-register before expiry (default 3600s, re-register at 3000s)
		time.Sleep(3000 * time.Second)
	}
}

// registerSIP sends a REGISTER request to the PBX with digest auth
func (gw *gateway) registerSIP() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Build REGISTER request
	reqURI := sip.Uri{Host: *sipDomain, Port: sipServerPort}
	fromHdr := sip.FromHeader{
		DisplayName: *sipUsername,
		Address:     sip.Uri{User: *sipUsername, Host: *sipDomain},
		Params:      sip.NewParams(),
	}
	fromHdr.Params.Add("tag", "pion-gw-register")

	toHdr := sip.ToHeader{
		DisplayName: *sipUsername,
		Address:     sip.Uri{User: *sipUsername, Host: *sipDomain},
	}

	contactHdr := &sip.ContactHeader{
		Address: sip.Uri{User: *sipUsername, Host: *publicIP, Port: *sipListenPort},
	}

	req := sip.NewRequest(sip.REGISTER, reqURI)
	req.AppendHeader(&fromHdr)
	req.AppendHeader(&toHdr)
	req.AppendHeader(contactHdr)
	req.AppendHeader(sip.NewHeader("Expires", "3600"))

	// Send REGISTER
	res, err := gw.sipClient.Do(ctx, req)
	if err != nil {
		return fmt.Errorf("REGISTER request: %w", err)
	}

	// Handle 401 Unauthorized (digest auth challenge)
	if res.StatusCode == 401 || res.StatusCode == 407 {
		res, err = gw.sipClient.DoDigestAuth(ctx, req, res, sipgo.DigestAuth{
			Username: *sipUsername,
			Password: *sipPassword,
		})
		if err != nil {
			return fmt.Errorf("REGISTER digest auth: %w", err)
		}
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("REGISTER failed with status %d", res.StatusCode)
	}

	return nil
}

// dialSIP sends an INVITE to the PBX for a target extension using the given agent credentials
func (gw *gateway) dialSIP(ws *threadSafeWriter, targetExtension string, localRTPPort int, agentExt, agentSIPPass string, parentCtx context.Context) (*sipCall, error) {
	ctx, cancel := context.WithTimeout(parentCtx, 60*time.Second)

	// Build SDP offer for the INVITE
	sdpOffer := buildSDPOffer(*publicIP, localRTPPort)
	sdpBytes, err := sdpOffer.Marshal()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("marshal SDP: %w", err)
	}
	log.Printf("SDP offer for INVITE (%d bytes, publicIP=%s, rtpPort=%d): %s", len(sdpBytes), *publicIP, localRTPPort, string(sdpBytes))

	// Build INVITE request using agent's extension as From
	reqURI := sip.Uri{User: targetExtension, Host: *sipDomain, Port: sipServerPort}
	fromHdr := sip.FromHeader{
		DisplayName: agentExt,
		Address:     sip.Uri{User: agentExt, Host: *sipDomain, Port: sipServerPort},
		Params:      sip.NewParams(),
	}
	fromHdr.Params.Add("tag", "pion-gw-"+fmt.Sprintf("%d", time.Now().UnixNano()))

	toHdr := sip.ToHeader{
		Address: sip.Uri{User: targetExtension, Host: *sipDomain, Port: sipServerPort},
	}

	contactHdr := &sip.ContactHeader{
		Address: sip.Uri{User: agentExt, Host: *publicIP, Port: *sipListenPort},
	}

	req := sip.NewRequest(sip.INVITE, reqURI)
	req.AppendHeader(&fromHdr)
	req.AppendHeader(&toHdr)
	req.AppendHeader(contactHdr)
	req.AppendHeader(&contentTypeHeaderSDP)
	req.SetBody(sdpBytes)

	// Notify frontend that the phone is ringing (play ringback tone immediately)
	gw.notifyRinging(ws)

	// Send INVITE using Do() which handles Via, CSeq, Call-ID, Max-Forwards automatically
	log.Printf("Sending SIP INVITE to %s", targetExtension)
	res, err := gw.sipClient.Do(ctx, req)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("SIP INVITE Do: %w", err)
	}
	log.Printf("INVITE initial response: status=%d", res.StatusCode)

	// Handle 401/407 digest auth challenge
	if res.StatusCode == 401 || res.StatusCode == 407 {
		log.Printf("INVITE auth challenge received (%d), doing digest auth", res.StatusCode)
		res, err = gw.sipClient.DoDigestAuth(ctx, req, res, sipgo.DigestAuth{
			Username: agentExt,
			Password: agentSIPPass,
		})
		if err != nil {
			cancel()
			return nil, fmt.Errorf("INVITE digest auth: %w", err)
		}
		log.Printf("INVITE digest auth response: status=%d", res.StatusCode)
	}

	if res.StatusCode != 200 {
		cancel()
		return nil, fmt.Errorf("INVITE failed with status %d", res.StatusCode)
	}

	// Send ACK for 2xx (must use res.To() with remote tag, route to Contact URI from 200 OK)
	// For 2xx ACK, the Request-URI should be the Contact URI from the 200 OK response
	ackReqURI := reqURI // default
	if contact := res.Contact(); contact != nil {
		ackReqURI = contact.Address
		log.Printf("ACK routing to Contact URI: %v", ackReqURI)
	}
	ackReq := sip.NewRequest(sip.ACK, ackReqURI)
	ackReq.AppendHeader(req.Via())
	ackReq.AppendHeader(req.From())
	ackReq.AppendHeader(res.To()) // Must use res.To() which has the remote tag
	ackReq.AppendHeader(req.CallID())
	ackReq.AppendHeader(sip.NewHeader("CSeq", fmt.Sprintf("%d ACK", req.CSeq().SeqNo)))
	ackReq.AppendHeader(sip.NewHeader("Max-Forwards", "70"))
	log.Printf("Sending ACK for INVITE to %v (To tag from response)", ackReqURI)
	if err := gw.sipClient.WriteRequest(ackReq); err != nil {
		log.Printf("Failed to send ACK: %v", err)
	}

	// Parse SDP answer to get remote RTP address
	log.Printf("SDP answer from PBX (%d bytes): %s", len(res.Body()), string(res.Body()))
	remoteAddr, err := parseSDPConnection(res.Body())
	if err != nil {
		log.Printf("Warning: failed to parse remote SDP: %v", err)
	} else {
		log.Printf("Remote RTP address from SDP: %s", remoteAddr)
	}

	// Cancel the INVITE timeout context (no longer needed after 200 OK)
	cancel()

	callID := req.CallID().String()

	// Extract dialog fields for proper BYE construction
	fromTagVal := ""
	if ft, ok := req.From().Params.Get("tag"); ok {
		fromTagVal = ft
	}
	toTagVal := ""
	if res.To() != nil {
		if tt, ok := res.To().Params.Get("tag"); ok {
			toTagVal = tt
		}
	}
	contactURI := sip.Uri{User: targetExtension, Host: *sipDomain, Port: sipServerPort}
	if contact := res.Contact(); contact != nil {
		contactURI = contact.Address
	}

	call := &sipCall{
		callID:     callID,
		from:       agentExt,
		to:         targetExtension,
		isOutbound: true,
		sdpAddr:    remoteAddr,
		remoteAddr: remoteAddr, // initial target, will be updated by RTP latching

		// Dialog fields for BYE
		fromTag:    fromTagVal,
		toTag:      toTagVal,
		cseqNo:     req.CSeq().SeqNo,
		contactURI: contactURI,
		agentExt:   agentExt,
		agentPass:  agentSIPPass,
	}

	log.Printf("Outbound SIP call established to %s (Call-ID: %s)", targetExtension, callID)
	return call, nil
}

// sendSIPBye sends a BYE to terminate an outbound SIP call using stored dialog fields
func (gw *gateway) sendSIPBye(call *sipCall) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use the Contact URI from 200 OK as Request-URI (standard SIP practice)
	reqURI := call.contactURI

	fromHdr := sip.FromHeader{
		Address: sip.Uri{User: call.agentExt, Host: *sipDomain, Port: sipServerPort},
		Params:  sip.NewParams(),
	}
	if call.fromTag != "" {
		fromHdr.Params.Add("tag", call.fromTag)
	}

	toHdr := sip.ToHeader{
		Address: sip.Uri{User: call.to, Host: *sipDomain, Port: sipServerPort},
		Params:  sip.NewParams(),
	}
	if call.toTag != "" {
		toHdr.Params.Add("tag", call.toTag)
	}

	req := sip.NewRequest(sip.BYE, reqURI)
	req.AppendHeader(&fromHdr)
	req.AppendHeader(&toHdr)
	req.AppendHeader(sip.NewHeader("Call-ID", call.callID))
	req.AppendHeader(sip.NewHeader("CSeq", fmt.Sprintf("%d BYE", call.cseqNo+1)))
	req.AppendHeader(sip.NewHeader("Max-Forwards", "70"))

	log.Printf("Sending BYE: Call-ID=%s From=%s;tag=%s To=%s;tag=%s CSeq=%d",
		call.callID, call.agentExt, call.fromTag, call.to, call.toTag, call.cseqNo+1)

	res, err := gw.sipClient.Do(ctx, req)
	if err != nil {
		return fmt.Errorf("BYE request: %w", err)
	}

	// Handle digest auth for BYE using agent credentials
	if res.StatusCode == 401 || res.StatusCode == 407 {
		res, err = gw.sipClient.DoDigestAuth(ctx, req, res, sipgo.DigestAuth{
			Username: call.agentExt,
			Password: call.agentPass,
		})
		if err != nil {
			return fmt.Errorf("BYE digest auth: %w", err)
		}
	}

	log.Printf("BYE response: %d", res.StatusCode)
	return nil
}

// startSIPServer is no longer used (server init moved to main)
// Kept as stub for compatibility
func (gw *gateway) startSIPServer() {}

// onSIPInvite handles incoming SIP INVITE requests
func (gw *gateway) onSIPInvite(req *sip.Request, tx sip.ServerTransaction) {
	callID := req.CallID().String()
	from := req.From().String()

	log.Printf("Incoming SIP INVITE from %s (Call-ID: %s)", from, callID)

	// Start RTP listener for this call
	rtpConn, rtpPort, err := startRTPListener()
	if err != nil {
		log.Printf("Failed to start RTP listener: %v", err)
		tx.Respond(sip.NewResponseFromRequest(req, 500, "Internal Server Error", nil))
		return
	}

	// Parse remote SDP to get RTP address
	remoteAddr, _ := parseSDPConnection(req.Body())

	// Sanitize callID for use in SDP (remove spaces, colons, etc.)
	safeCallID := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, strings.TrimPrefix(callID, "Call-ID: "))

	// Create audio track for SIP→WebRTC direction
	audioTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypePCMU},
		fmt.Sprintf("sip-in-%s", safeCallID),
		"pion-sip",
	)
	if err != nil {
		log.Printf("Failed to create audio track: %v", err)
		rtpConn.Close()
		tx.Respond(sip.NewResponseFromRequest(req, 500, "Internal Server Error", nil))
		return
	}

	// Extract dialog fields from the inbound INVITE for BYE construction
	inFromTag := ""
	if ft, ok := req.From().Params.Get("tag"); ok {
		inFromTag = ft
	}
	inToTag := ""
	// We add our own To tag when sending 200 OK - generate one
	inToTag = "gw-" + fmt.Sprintf("%d", time.Now().UnixNano())
	inCseqNo := uint32(0)
	if req.CSeq() != nil {
		inCseqNo = req.CSeq().SeqNo
	}
	// Contact URI for BYE: use the Contact from the INVITE (where the caller wants to receive requests)
	inContactURI := sip.Uri{Host: *sipDomain, Port: sipServerPort}
	if contact := req.Contact(); contact != nil {
		inContactURI = contact.Address
	}
	// For inbound BYE: we are the "To" party, caller is the "From" party
	// Our From = the INVITE's To (our side), Our To = the INVITE's From (caller)
	inFromUser := ""
	if req.To() != nil {
		inFromUser = req.To().Address.User
	}
	inToUser := ""
	if req.From() != nil {
		inToUser = req.From().Address.User
	}

	call := &sipCall{
		rtpConn:    rtpConn,
		sdpAddr:    remoteAddr,
		remoteAddr: remoteAddr, // initial target, will be updated by RTP latching
		from:       from,
		callID:     callID,
		audioTrack: audioTrack,

		// Dialog fields for BYE (inbound: we are UAS, caller is UAC)
		fromTag:    inToTag,   // Our tag (the To tag we assigned in 200 OK)
		toTag:      inFromTag, // Caller's tag (the From tag from INVITE)
		cseqNo:     inCseqNo,
		contactURI: inContactURI,
		agentExt:   inFromUser, // Our side (the To party of the INVITE)
		agentPass:  *sipPassword,
		to:         inToUser, // The caller (the From party of the INVITE)
	}

	// Store the call
	gw.mu.Lock()
	gw.calls[callID] = call
	gw.mu.Unlock()

	// Forward RTP packets from SIP to the audio track
	ctx, cancel := context.WithCancel(context.Background())
	call.cancelFunc = cancel
	go gw.forwardRTPToTrack(ctx, call)

	// Send call-started event to all connected browsers
	gw.broadcastToPeers(wsMessage{Event: "call-started", Data: from})

	// Add the audio track to all connected WebRTC peers
	gw.addTrackToAllPeers(audioTrack)

	// Generate SDP answer for the SIP INVITE
	sdpAnswer := generateSDPAnswer(req.Body(), *publicIP, rtpPort)

	res := sip.NewResponseFromRequest(req, 200, "OK", sdpAnswer)
	res.AppendHeader(&sip.ContactHeader{Address: sip.Uri{Host: *publicIP, Port: *sipListenPort}})
	res.AppendHeader(&contentTypeHeaderSDP)
	// Add our To tag to the 200 OK response (required for SIP dialog)
	if res.To() != nil {
		res.To().Params.Add("tag", inToTag)
	}

	if err := tx.Respond(res); err != nil {
		log.Printf("Failed to respond to INVITE: %v", err)
		return
	}

	log.Printf("Inbound SIP call established: %s (RTP port: %d)", from, rtpPort)
}

// onSIPBye handles SIP BYE requests (call termination)
func (gw *gateway) onSIPBye(req *sip.Request, tx sip.ServerTransaction) {
	callID := req.CallID().String()
	log.Printf("SIP BYE received for Call-ID: %s", callID)

	if err := tx.Respond(sip.NewResponseFromRequest(req, 200, "OK", nil)); err != nil {
		log.Printf("Failed to respond to BYE: %v", err)
	}

	gw.endCall(callID)
	gw.broadcastToPeers(wsMessage{Event: "call-ended", Data: ""})
}

// onSIPAck handles SIP ACK requests
func (gw *gateway) onSIPAck(req *sip.Request, tx sip.ServerTransaction) {
	if err := tx.Respond(sip.NewResponseFromRequest(req, 200, "OK", nil)); err != nil {
		log.Printf("Failed to respond to ACK: %v", err)
	}
}

// startRTPListener creates a UDP listener on a port from the configured RTP port range
// This allows server firewalls to open only a specific range of UDP ports
var rtpPortCounter int
var rtpPortMu sync.Mutex

func startRTPListener() (*net.UDPConn, int, error) {
	rtpPortMu.Lock()
	defer rtpPortMu.Unlock()

	min := *rtpPortMin
	max := *rtpPortMax
	if min > max || min < 1 || max > 65535 {
		return nil, 0, fmt.Errorf("invalid RTP port range: %d-%d", min, max)
	}

	// Cycle through ports in the range, starting from last used + 1
	for i := 0; i <= (max - min); i++ {
		port := min + (rtpPortCounter+i)%(max-min+1)
		conn, err := net.ListenUDP("udp", &net.UDPAddr{
			Port: port,
			IP:   net.ParseIP("0.0.0.0"),
		})
		if err == nil {
			// Increase UDP receive buffer to prevent OS-level packet drops (choppy audio)
			// Default is ~8KB which overflows quickly; 4MB holds ~2 seconds of PCMU audio
			if err := conn.SetReadBuffer(4 * 1024 * 1024); err != nil {
				log.Printf("Warning: failed to increase UDP read buffer: %v", err)
			}
			if err := conn.SetWriteBuffer(4 * 1024 * 1024); err != nil {
				log.Printf("Warning: failed to increase UDP write buffer: %v", err)
			}
			rtpPortCounter = (port - min + 1) % (max - min + 1)
			log.Printf("RTP listener started on UDP port %d", port)
			return conn, port, nil
		}
		// Port in use, try next
	}

	return nil, 0, fmt.Errorf("no available RTP ports in range %d-%d", min, max)
}

// forwardRTPToTrack reads RTP packets from SIP and writes them to the WebRTC audio track
// Implements RTP latching: updates remoteAddr from actual incoming packet source (handles NAT)
func (gw *gateway) forwardRTPToTrack(ctx context.Context, call *sipCall) {
	buff := make([]byte, 1500)
	pktCount := 0
	var lastSeq uint16
	seqInit := false
	for {
		select {
		case <-ctx.Done():
			log.Printf("SIP→WebRTC forwarding stopped for call %s (%d packets)", call.callID, pktCount)
			return
		default:
		}

		// Set read deadline so we can check context cancellation
		call.rtpConn.SetReadDeadline(time.Now().Add(1 * time.Second))

		n, addr, err := call.rtpConn.ReadFromUDP(buff)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			if ctx.Err() != nil {
				return
			}
			log.Printf("RTP read error for call %s: %v", call.callID, err)
			return
		}

		// Filter out RTCP packets (payload type >= 200 in byte 1 low 7 bits)
		// RTCP mixed with RTP on the same port causes choppy/corrupted audio
		if n >= 2 {
			pt := buff[1] & 0x7F
			if pt >= 200 {
				continue // skip RTCP Sender Report, Receiver Report, etc.
			}
			// Filter comfort noise (PT=13) and telephone-event/DTMF (PT=101 or dynamic 96-127)
			// These are NOT PCMU audio and cause "ssshhh" static when written to WebRTC track
			if pt == 13 || pt == 101 || (pt >= 96 && pt != 0) {
				continue
			}
		}

		// Drop first 10 RTP packets - they often contain ringback tail/noise from PBX
		// which causes the "ssshhh" static sound at call start
		pktCount++
		if pktCount <= 10 {
			if pktCount == 1 {
				log.Printf("Dropping first 10 RTP packets (noise filter)")
			}
			continue
		}

		// RTP latching: use actual source address as destination for outgoing RTP
		if !call.latched {
			call.latched = true
			call.remoteAddr = addr
			log.Printf("RTP latched: remoteAddr updated to %s (was SDP addr %s)", addr, call.sdpAddr)
		}

		// Track sequence numbers to detect gaps (packet loss causes choppy audio)
		if n >= 4 {
			seq := uint16(buff[2])<<8 | uint16(buff[3])
			if seqInit {
				gap := seq - lastSeq
				if gap > 1 && gap < 30000 {
					log.Printf("RTP seq gap: expected %d got %d (gap=%d) - packet loss detected", lastSeq+1, seq, gap-1)
				}
			}
			lastSeq = seq
			seqInit = true
		}

		if pktCount <= 15 || pktCount%500 == 0 {
			log.Printf("SIP→WebRTC: pkt #%d, %d bytes from %s", pktCount, n, addr)
		}

		if _, err := call.audioTrack.Write(buff[:n]); err != nil {
			// Don't return on single write error - WebRTC track may recover
			if pktCount%100 != 0 {
				log.Printf("Audio track write error for call %s: %v", call.callID, err)
			}
		}
	}
}

// forwardTrackToRTP reads RTP packets from a WebRTC remote track and sends them to the SIP remote RTP address
// Uses call.remoteAddr which is updated via RTP latching once the first SIP→RTP packet arrives
func forwardTrackToRTP(track *webrtc.TrackRemote, call *sipCall) {
	buff := make([]byte, 1500)
	pktCount := 0
	for {
		n, _, err := track.Read(buff)
		if err != nil {
			log.Printf("WebRTC track read error: %v", err)
			return
		}

		// Use latched address if available, otherwise fall back to SDP address
		dest := call.remoteAddr
		if dest == nil {
			dest = call.sdpAddr
		}

		pktCount++
		if pktCount <= 5 || pktCount%500 == 0 {
			log.Printf("WebRTC→SIP: pkt #%d, %d bytes to %s (latched=%v)", pktCount, n, dest, call.latched)
		}

		if dest != nil {
			if _, err := call.rtpConn.WriteToUDP(buff[:n], dest); err != nil {
				// Don't return on single write error - network may recover
				if pktCount%100 != 0 {
					log.Printf("RTP write error: %v", err)
				}
			}
		} else if pktCount <= 3 {
			log.Printf("WebRTC→SIP: dropping pkt #%d, no remote address (no SDP addr, not latched yet)", pktCount)
		}
	}
}

// endCall cleans up a SIP call
func (gw *gateway) endCall(callID string) {
	gw.mu.Lock()
	call, ok := gw.calls[callID]
	if !ok {
		gw.mu.Unlock()
		return
	}
	delete(gw.calls, callID)

	// Remove call association from any peer that has it, and swap back to placeholder track
	for _, peer := range gw.peers {
		if peer.call != nil && peer.call.callID == callID {
			peer.call = nil
			// Replace SIP audio track back with placeholder (no renegotiation needed)
			if call.audioTrack != nil && peer.placeholderTrack != nil {
				senders := peer.pc.GetSenders()
				if len(senders) > 0 {
					if err := senders[0].ReplaceTrack(peer.placeholderTrack); err != nil {
						log.Printf("Failed to replace track back to placeholder: %v", err)
					} else {
						log.Printf("Replaced SIP track back to placeholder (no renegotiation)")
					}
				}
			}
		}
	}
	gw.mu.Unlock()

	// Cancel the RTP forwarding context
	if call.cancelFunc != nil {
		call.cancelFunc()
	}

	// Close the RTP connection
	if call.rtpConn != nil {
		call.rtpConn.Close()
	}

	log.Printf("SIP call ended: %s", callID)
}

// notifyRinging sends a ringing event to the browser peer
func (gw *gateway) notifyRinging(ws *threadSafeWriter) {
	ws.WriteJSON(wsMessage{Event: "ringing", Data: ""})
}

// addTrackToAllPeers adds an audio track to every connected WebRTC PeerConnection
func (gw *gateway) addTrackToAllPeers(track *webrtc.TrackLocalStaticRTP) {
	gw.mu.RLock()
	defer gw.mu.RUnlock()

	for _, peer := range gw.peers {
		senders := peer.pc.GetSenders()
		if len(senders) > 0 {
			if err := senders[0].ReplaceTrack(track); err != nil {
				log.Printf("Failed to replace track on peer sender: %v", err)
			}
		} else {
			if _, err := peer.pc.AddTrack(track); err != nil {
				log.Printf("Failed to add track to peer: %v", err)
				continue
			}
		}
		gw.renegotiatePeer(peer)
	}
}

// removeTrackFromAllPeers removes an audio track from every connected WebRTC PeerConnection
func (gw *gateway) removeTrackFromAllPeers(track *webrtc.TrackLocalStaticRTP) {
	gw.mu.RLock()
	defer gw.mu.RUnlock()

	for _, peer := range gw.peers {
		for _, sender := range peer.pc.GetSenders() {
			if sender.Track() != nil && sender.Track().ID() == track.ID() {
				if err := peer.pc.RemoveTrack(sender); err != nil {
					log.Printf("Failed to remove track from peer: %v", err)
				}
				gw.renegotiatePeer(peer)
				break
			}
		}
	}
}

// renegotiatePeer creates an offer and sends it to the WebRTC peer via WebSocket
// It is safe to call multiple times - if already negotiating, it sets pendingRenegotiate flag
func (gw *gateway) renegotiatePeer(peer *peerState) {
	peer.negotiateMu.Lock()
	if peer.negotiating {
		log.Printf("Renegotiation requested while already in progress - deferring")
		peer.pendingRenegotiate = true
		peer.negotiateMu.Unlock()
		return
	}
	peer.negotiating = true
	peer.pendingRenegotiate = false
	peer.negotiateMu.Unlock()

	go func() {
		offer, err := peer.pc.CreateOffer(nil)
		if err != nil {
			log.Printf("Failed to create offer: %v", err)
			peer.negotiateMu.Lock()
			peer.negotiating = false
			peer.negotiateMu.Unlock()
			return
		}

		// Log offer SDP to verify it contains a=sendrecv (not a=sendonly)
		log.Printf("Offer SDP (%d bytes): %s", len(offer.SDP), offer.SDP)
		for i, tr := range peer.pc.GetTransceivers() {
			log.Printf("Offer transceiver[%d]: direction=%s", i, tr.Direction())
		}

		if err := peer.pc.SetLocalDescription(offer); err != nil {
			log.Printf("Failed to set local description: %v", err)
			peer.negotiateMu.Lock()
			peer.negotiating = false
			peer.negotiateMu.Unlock()
			return
		}

		// Wait for ICE gathering to complete
		gatherComplete := webrtc.GatheringCompletePromise(peer.pc)
		<-gatherComplete

		offerJSON, err := json.Marshal(peer.pc.LocalDescription())
		if err != nil {
			log.Printf("Failed to marshal offer: %v", err)
			peer.negotiateMu.Lock()
			peer.negotiating = false
			peer.negotiateMu.Unlock()
			return
		}

		msg := wsMessage{Event: "offer", Data: string(offerJSON)}
		if err := peer.ws.WriteJSON(msg); err != nil {
			log.Printf("Failed to send offer via WebSocket: %v", err)
			peer.negotiateMu.Lock()
			peer.negotiating = false
			peer.negotiateMu.Unlock()
			return
		}
		// negotiating stays true until the browser sends an answer
	}()
}

// broadcastToPeers sends a message to all connected WebSocket peers
func (gw *gateway) broadcastToPeers(msg wsMessage) {
	gw.mu.RLock()
	defer gw.mu.RUnlock()

	for _, peer := range gw.peers {
		if err := peer.ws.WriteJSON(msg); err != nil {
			log.Printf("Failed to broadcast to peer: %v", err)
		}
	}
}

// corsMiddleware adds CORS headers and handles preflight OPTIONS requests
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// startHTTPServer serves the web UI and handles WebSocket connections
func (gw *gateway) startHTTPServer() {
	mux := http.NewServeMux()

	// Register REST API routes
	registerAPIRoutes(mux)

	// WebSocket endpoint
	mux.HandleFunc("/ws", gw.handleWebSocket)

	// Serve CRM frontend (static files from ./crm-dist/ or fallback index.html)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Try CRM dist folder first
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			if _, err := os.Stat("crm-dist/index.html"); err == nil {
				http.ServeFile(w, r, "crm-dist/index.html")
				return
			}
		}
		// Serve static assets from crm-dist
		if strings.HasPrefix(r.URL.Path, "/assets/") || strings.HasPrefix(r.URL.Path, "/static/") {
			filePath := "crm-dist" + r.URL.Path
			if _, err := os.Stat(filePath); err == nil {
				http.ServeFile(w, r, filePath)
				return
			}
		}
		// Fallback to old index.html
		http.ServeFile(w, r, "index.html")
	})

	// Wrap mux with CORS middleware
	corsHandler := corsMiddleware(mux)

	addr := fmt.Sprintf(":%d", *httpPort)

	if *enableTLS {
		// Use provided cert/key or auto-generate self-signed cert
		if *tlsCert != "" && *tlsKey != "" {
			log.Printf("Starting HTTPS server on %s (using provided cert)", addr)
			if err := http.ListenAndServeTLS(addr, *tlsCert, *tlsKey, corsHandler); err != nil {
				log.Fatalf("HTTPS server failed: %v", err)
			}
		} else {
			certFile := "cert.pem"
			keyFile := "key.pem"
			if _, err := os.Stat(certFile); os.IsNotExist(err) {
				log.Printf("Auto-generating self-signed TLS certificate...")
				if err := generateSelfSignedCert(certFile, keyFile); err != nil {
					log.Fatalf("Failed to generate self-signed cert: %v", err)
				}
				log.Printf("Self-signed cert generated: %s, %s", certFile, keyFile)
			}
			log.Printf("Starting HTTPS server on %s (self-signed cert - accept browser warning!)", addr)
			if err := http.ListenAndServeTLS(addr, certFile, keyFile, corsHandler); err != nil {
				log.Fatalf("HTTPS server failed: %v", err)
			}
		}
	} else {
		log.Printf("Starting HTTP server on %s (use -tls for HTTPS/browser mic access)", addr)
		if err := http.ListenAndServe(addr, corsHandler); err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}
}

// generateSelfSignedCert creates a self-signed TLS certificate for development
func generateSelfSignedCert(certFile, keyFile string) error {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour)

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}

	template := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{Organization: []string{"SIP Gateway Dev"}},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1"), net.ParseIP(*publicIP)},
		DNSNames:              []string{"localhost"},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return err
	}

	certOut, err := os.Create(certFile)
	if err != nil {
		return err
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return err
	}

	keyOut, err := os.Create(keyFile)
	if err != nil {
		return err
	}
	defer keyOut.Close()
	if err := pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}); err != nil {
		return err
	}

	return nil
}

// handleWebSocket upgrades HTTP to WebSocket and manages WebRTC PeerConnection
func (gw *gateway) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer ws.Close()

	log.Printf("New WebSocket connection from %s", ws.RemoteAddr())

	// Create a MediaEngine that ONLY supports PCMU (G.711u) to match PBX codec
	// Without this, browser negotiates Opus which PBX can't decode
	m := &webrtc.MediaEngine{}
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypePCMU, ClockRate: 8000, Channels: 1},
		PayloadType:        0,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		log.Printf("Failed to register PCMU codec: %v", err)
		return
	}

	// Create API with custom MediaEngine
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m))

	// Create PCMU-only PeerConnection
	pc, err := api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	})
	if err != nil {
		log.Printf("Failed to create PCMU-only PeerConnection: %v", err)
		return
	}
	defer pc.Close()

	// Create a placeholder audio track so the sender always has a track.
	// This allows handleDial to use ReplaceTrack (track→track swap) without renegotiation.
	// Without a placeholder, ReplaceTrack from nil→track requires renegotiation which breaks audio.
	placeholderTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypePCMU},
		"placeholder", "pion-placeholder",
	)
	if err != nil {
		log.Printf("Failed to create placeholder track: %v", err)
		return
	}

	// Add sendrecv audio transceiver with the placeholder track.
	// handleDial will use ReplaceTrack to swap it with the real SIP audio track.
	// This ensures ONE audio m-line with a=sendrecv in the offer.
	if _, err := pc.AddTransceiverFromTrack(placeholderTrack,
		webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendrecv},
	); err != nil {
		log.Printf("Failed to add audio transceiver: %v", err)
		return
	}

	// Wrap WebSocket for thread-safe writes
	safeWS := &threadSafeWriter{Conn: ws}

	// ICE candidate handler
	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}
		candidateJSON, err := json.Marshal(candidate.ToJSON())
		if err != nil {
			log.Printf("Failed to marshal ICE candidate: %v", err)
			return
		}
		msg := wsMessage{Event: "candidate", Data: string(candidateJSON)}
		if err := safeWS.WriteJSON(msg); err != nil {
			log.Printf("Failed to send ICE candidate: %v", err)
		}
	})

	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Printf("ICE connection state: %s", state.String())
	})

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("PeerConnection state: %s", state.String())
		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed {
			gw.removePeer(safeWS)
		}
	})

	// Register the peer BEFORE setting OnTrack so the handler can reference it
	peer := &peerState{ws: safeWS, pc: pc, callReady: make(chan struct{}, 1), placeholderTrack: placeholderTrack}
	gw.mu.Lock()
	gw.peers[safeWS] = peer

	// For existing inbound SIP calls, replace the generated track on the sender
	for _, call := range gw.calls {
		if call.audioTrack != nil {
			senders := pc.GetSenders()
			if len(senders) > 0 {
				if err := senders[0].ReplaceTrack(call.audioTrack); err != nil {
					log.Printf("Failed to replace track on new peer sender: %v", err)
				}
			}
		}
	}
	gw.mu.Unlock()

	// Handle incoming audio track from browser (for outbound calls)
	// Uses channel signal instead of busy-wait for lower latency and CPU usage
	pc.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		log.Printf("Got remote audio track from browser: Kind=%s, ID=%s", track.Kind(), track.ID())

		for {
			select {
			case <-peer.callReady:
				gw.mu.RLock()
				call := peer.call
				gw.mu.RUnlock()
				if call != nil && call.rtpConn != nil {
					forwardTrackToRTP(track, call)
					return
				}
			case <-time.After(30 * time.Second):
				gw.mu.RLock()
				_, ok := gw.peers[safeWS]
				gw.mu.RUnlock()
				if !ok {
					return
				}
			}
		}
	})

	// Send initial offer to browser so ICE can establish.
	// The transceiver has a generated track (placeholder) - handleDial will
	// ReplaceTrack with the real SIP audio track and renegotiate.
	gw.renegotiatePeer(peer)

	// Main WebSocket read loop
	for {
		_, raw, err := safeWS.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			gw.removePeer(safeWS)
			return
		}

		var msg wsMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			log.Printf("Failed to unmarshal WebSocket message: %v", err)
			continue
		}

		switch msg.Event {
		case "answer":
			var answer webrtc.SessionDescription
			if err := json.Unmarshal([]byte(msg.Data), &answer); err != nil {
				log.Printf("Failed to unmarshal answer: %v", err)
				continue
			}
			if err := pc.SetRemoteDescription(answer); err != nil {
				log.Printf("Failed to set remote description: %v", err)
			} else {
				// Log SDP answer to verify browser is sending audio (look for SSRC lines)
				log.Printf("SDP answer from browser (%d bytes): %s", len(answer.SDP), answer.SDP)
				for i, tr := range pc.GetTransceivers() {
					log.Printf("Transceiver[%d]: direction=%s senderTrack=%v receiverTrack=%v",
						i, tr.Direction(), tr.Sender() != nil && tr.Sender().Track() != nil,
						tr.Receiver() != nil && tr.Receiver().Track() != nil)
				}
				log.Printf("WebRTC answer set successfully, signaling state: %s", pc.SignalingState())
				// Clear negotiating flag and trigger pending renegotiation if needed
				peer.negotiateMu.Lock()
				peer.negotiating = false
				shouldRenegotiate := peer.pendingRenegotiate
				peer.pendingRenegotiate = false
				peer.negotiateMu.Unlock()
				if shouldRenegotiate {
					log.Printf("Triggering deferred renegotiation")
					gw.renegotiatePeer(peer)
				}
			}

		case "candidate":
			var candidate webrtc.ICECandidateInit
			if err := json.Unmarshal([]byte(msg.Data), &candidate); err != nil {
				log.Printf("Failed to unmarshal candidate: %v", err)
				continue
			}
			if err := pc.AddICECandidate(candidate); err != nil {
				log.Printf("Failed to add ICE candidate: %v", err)
			}

		case "auth":
			// Browser sends login token to associate this WebSocket with an agent
			token := msg.Data
			sess, err := parseToken(token)
			if err != nil {
				log.Printf("WebSocket auth failed: %v", err)
				safeWS.WriteJSON(wsMessage{Event: "auth-error", Data: "invalid token"})
				continue
			}
			agent, err := getAgentByID(sess.AgentID)
			if err != nil {
				safeWS.WriteJSON(wsMessage{Event: "auth-error", Data: "agent not found"})
				continue
			}
			peer.agentID = agent.ID
			peer.agentExt = agent.Extension
			peer.agentSIPPass = agent.SIPPassword
			peer.token = token
			log.Printf("WebSocket authenticated: agent=%s ext=%s", agent.Username, agent.Extension)
			safeWS.WriteJSON(wsMessage{Event: "auth-ok", Data: fmt.Sprintf("%s:%s", agent.Username, agent.Extension)})

		case "dial":
			// Browser wants to make an outbound SIP call
			// msg.Data format: "extension" or "extension:customerID"
			dialData := msg.Data
			extension := dialData
			var customerID *int64
			if parts := strings.SplitN(dialData, ":", 2); len(parts) == 2 {
				extension = parts[0]
				if cid, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
					customerID = &cid
				}
			}
			log.Printf("Dial request for extension: %s (agentID=%d, customerID=%v)", extension, peer.agentID, customerID)
			go gw.handleDial(safeWS, peer, extension, customerID)

		case "hangup":
			log.Printf("Hangup request from browser")
			gw.handleHangup(peer)

		default:
			log.Printf("Unknown WebSocket event: %s", msg.Event)
		}
	}
}

// handleDial processes a dial request from the browser
func (gw *gateway) handleDial(ws *threadSafeWriter, peer *peerState, extension string, customerID *int64) {
	gw.mu.Lock()
	if peer.call != nil || peer.dialing {
		gw.mu.Unlock()
		ws.WriteJSON(wsMessage{Event: "dial-error", Data: "Already in a call"})
		return
	}
	peer.dialing = true
	dialCtx, dialCancel := context.WithCancel(context.Background())
	peer.dialCancel = dialCancel
	gw.mu.Unlock()

	// Ensure cleanup on any exit path
	defer func() {
		gw.mu.Lock()
		peer.dialing = false
		peer.dialCancel = nil
		gw.mu.Unlock()
	}()

	if gw.sipClient == nil {
		ws.WriteJSON(wsMessage{Event: "dial-error", Data: "SIP client not configured"})
		return
	}

	if peer.agentID == 0 {
		ws.WriteJSON(wsMessage{Event: "dial-error", Data: "Not authenticated - send auth event first"})
		return
	}

	// Create call log in DB
	callLogID, err := createCallLog(peer.agentID, customerID, extension, "outbound")
	if err != nil {
		log.Printf("Warning: failed to create call log: %v", err)
	} else {
		log.Printf("Call log created: id=%d agent=%d ext=%s", callLogID, peer.agentID, extension)
	}

	// Start RTP listener for outbound call
	rtpConn, rtpPort, err := startRTPListener()
	if err != nil {
		ws.WriteJSON(wsMessage{Event: "dial-error", Data: fmt.Sprintf("RTP listener error: %v", err)})
		return
	}

	// Create audio track BEFORE sending INVITE so the WebRTC audio path is ready
	// immediately when the 200 OK arrives (eliminates connection delay).
	audioTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypePCMU},
		fmt.Sprintf("sip-out-%d", time.Now().UnixNano()),
		"pion-sip-out",
	)
	if err != nil {
		log.Printf("Failed to create outbound audio track: %v", err)
		rtpConn.Close()
		ws.WriteJSON(wsMessage{Event: "dial-error", Data: "Failed to create audio track"})
		return
	}

	// Replace the placeholder track on the existing sendrecv transceiver's sender.
	// Since the sender already has a track (placeholder), ReplaceTrack is a simple swap
	// that does NOT require renegotiation — audio path stays intact.
	senders := peer.pc.GetSenders()
	if len(senders) > 0 {
		if err := senders[0].ReplaceTrack(audioTrack); err != nil {
			log.Printf("Failed to replace track on sender: %v", err)
		} else {
			log.Printf("Replaced placeholder track with SIP audio track (before INVITE)")
		}
	} else {
		log.Printf("WARNING: no sender found, falling back to AddTrack")
		if _, err := peer.pc.AddTrack(audioTrack); err != nil {
			log.Printf("Failed to add outbound audio track: %v", err)
		}
		gw.renegotiatePeer(peer)
	}

	// Create partial call object for forwardRTPToTrack (fills in after dialSIP returns)
	call := &sipCall{
		rtpConn:    rtpConn,
		audioTrack: audioTrack,
		isOutbound: true,
		agentExt:   peer.agentExt,
		agentPass:  peer.agentSIPPass,
	}

	// Start forwarding RTP from SIP to WebRTC track BEFORE INVITE
	// so packets flow immediately when the PBX starts sending after 200 OK
	ctx, cancel := context.WithCancel(context.Background())
	call.cancelFunc = cancel
	go gw.forwardRTPToTrack(ctx, call)

	// Send INVITE to PBX using agent's SIP credentials (blocks until 200 OK or error)
	// Uses dialCtx so hangup during dial cancels the INVITE
	sipCall, err := gw.dialSIP(ws, extension, rtpPort, peer.agentExt, peer.agentSIPPass, dialCtx)
	if err != nil {
		cancel()
		// Swap back to placeholder track since dial failed
		if peer.placeholderTrack != nil {
			senders := peer.pc.GetSenders()
			if len(senders) > 0 {
				senders[0].ReplaceTrack(peer.placeholderTrack)
			}
		}
		rtpConn.Close()
		ws.WriteJSON(wsMessage{Event: "dial-error", Data: fmt.Sprintf("SIP INVITE failed: %v", err)})
		return
	}

	// Fill in SIP dialog fields from the successful INVITE
	call.callID = sipCall.callID
	call.from = sipCall.from
	call.to = sipCall.to
	call.sdpAddr = sipCall.sdpAddr
	call.remoteAddr = sipCall.remoteAddr
	call.fromTag = sipCall.fromTag
	call.toTag = sipCall.toTag
	call.cseqNo = sipCall.cseqNo
	call.contactURI = sipCall.contactURI

	// Send NAT keepalive packets to open the RTP pinhole
	// The PBX needs to receive a packet from us first so its NAT/firewall allows return traffic
	if call.sdpAddr != nil {
		go func() {
			// Empty RTP packet (minimal valid header: version=2, PT=0/PCMU, seq=0, ts=0, ssrc=0)
			keepalive := []byte{0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if call.latched {
						return // RTP flow established, no more keepalives needed
					}
					if _, err := rtpConn.WriteToUDP(keepalive, call.sdpAddr); err != nil {
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// Set CRM fields on call
	call.callLogID = callLogID
	call.agentID = peer.agentID
	call.customerID = customerID
	call.startedAt = time.Now()

	// Update call log to answered
	if callLogID > 0 {
		if err := updateCallStatus(callLogID, "answered", 0); err != nil {
			log.Printf("Warning: failed to update call log: %v", err)
		}
	}

	// Associate call with peer and signal the OnTrack handler
	// Also store in global calls map (was previously done in dialSIP)
	gw.mu.Lock()
	peer.call = call
	gw.calls[call.callID] = call
	gw.mu.Unlock()
	select {
	case peer.callReady <- struct{}{}:
	default:
		// Channel already has a signal, no need to block
	}

	ws.WriteJSON(wsMessage{Event: "call-started", Data: extension})
	log.Printf("Outbound call to %s established for agent %d", extension, peer.agentID)
}

// handleHangup terminates the SIP call for a browser peer
func (gw *gateway) handleHangup(peer *peerState) {
	gw.mu.Lock()
	call := peer.call
	peer.call = nil

	// If a dial is in progress (INVITE not yet completed), cancel it
	if peer.dialing && peer.dialCancel != nil {
		log.Printf("Cancelling in-progress dial")
		peer.dialCancel()
		peer.dialing = false
		peer.dialCancel = nil
	}
	gw.mu.Unlock()

	if call == nil {
		// No active call, but we already cancelled any in-progress dial above
		// Notify browser that call ended
		peer.ws.WriteJSON(wsMessage{Event: "call-ended", Data: "cancelled"})
		return
	}

	// Update call log with duration
	if call.callLogID > 0 && !call.startedAt.IsZero() {
		duration := int(time.Since(call.startedAt).Seconds())
		status := "answered"
		if duration == 0 {
			status = "no-answer"
		}
		if err := updateCallStatus(call.callLogID, status, duration); err != nil {
			log.Printf("Warning: failed to update call log: %v", err)
		}
		log.Printf("Call log updated: id=%d status=%s duration=%ds", call.callLogID, status, duration)
	}

	// Send BYE to PBX for both outbound and inbound calls
	if gw.sipClient != nil {
		if err := gw.sendSIPBye(call); err != nil {
			log.Printf("Failed to send BYE: %v", err)
		}
	}

	// Notify browser that call ended
	peer.ws.WriteJSON(wsMessage{Event: "call-ended", Data: "hangup"})

	gw.endCall(call.callID)
}

// removePeer cleans up a disconnected WebRTC peer
func (gw *gateway) removePeer(ws *threadSafeWriter) {
	gw.mu.Lock()
	defer gw.mu.Unlock()

	peer, ok := gw.peers[ws]
	if !ok {
		return
	}

	// Clean up any active call
	if peer.call != nil {
		if peer.call.isOutbound && gw.sipClient != nil {
			go gw.sendSIPBye(peer.call)
		}
		go gw.endCall(peer.call.callID)
	}

	peer.pc.Close()
	delete(gw.peers, ws)
	log.Printf("Peer removed: %s", ws.RemoteAddr())
}

// buildSDPOffer creates a SIP SDP offer for an outbound INVITE
func buildSDPOffer(unicastAddress string, rtpPort int) sdp.SessionDescription {
	return sdp.SessionDescription{
		Version: 0,
		Origin: sdp.Origin{
			Username:       "-",
			SessionID:      123456789,
			SessionVersion: 123456789 + 2,
			NetworkType:    "IN",
			AddressType:    "IP4",
			UnicastAddress: unicastAddress,
		},
		SessionName: "Pion-SIP-Gateway",
		ConnectionInformation: &sdp.ConnectionInformation{
			NetworkType: "IN",
			AddressType: "IP4",
			Address:     &sdp.Address{Address: unicastAddress},
		},
		TimeDescriptions: []sdp.TimeDescription{
			{Timing: sdp.Timing{StartTime: 0, StopTime: 0}},
		},
		MediaDescriptions: []*sdp.MediaDescription{
			{
				MediaName: sdp.MediaName{
					Media:   "audio",
					Port:    sdp.RangedPort{Value: rtpPort},
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"0"},
				},
				Attributes: []sdp.Attribute{
					{Key: "rtpmap", Value: "0 PCMU/8000"},
					{Key: "ptime", Value: "20"},
					{Key: "maxptime", Value: "150"},
					{Key: "sendrecv"},
				},
			},
		},
	}
}

// generateSDPAnswer creates a SIP SDP answer for an inbound INVITE
func generateSDPAnswer(offerBody []byte, unicastAddress string, rtpPort int) []byte {
	offerParsed := sdp.SessionDescription{}
	if err := offerParsed.Unmarshal(offerBody); err != nil {
		log.Printf("Failed to parse SDP offer: %v", err)
	}

	answer := sdp.SessionDescription{
		Version: 0,
		Origin: sdp.Origin{
			Username:       "-",
			SessionID:      offerParsed.Origin.SessionID,
			SessionVersion: offerParsed.Origin.SessionID + 2,
			NetworkType:    "IN",
			AddressType:    "IP4",
			UnicastAddress: unicastAddress,
		},
		SessionName: "Pion-SIP-Gateway",
		ConnectionInformation: &sdp.ConnectionInformation{
			NetworkType: "IN",
			AddressType: "IP4",
			Address:     &sdp.Address{Address: unicastAddress},
		},
		TimeDescriptions: []sdp.TimeDescription{
			{Timing: sdp.Timing{StartTime: 0, StopTime: 0}},
		},
		MediaDescriptions: []*sdp.MediaDescription{
			{
				MediaName: sdp.MediaName{
					Media:   "audio",
					Port:    sdp.RangedPort{Value: rtpPort},
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"0"},
				},
				Attributes: []sdp.Attribute{
					{Key: "rtpmap", Value: "0 PCMU/8000"},
					{Key: "ptime", Value: "20"},
					{Key: "maxptime", Value: "150"},
					{Key: "sendrecv"},
				},
			},
		},
	}

	answerByte, err := answer.Marshal()
	if err != nil {
		log.Printf("Failed to marshal SDP answer: %v", err)
		return nil
	}

	return answerByte
}

// parseSDPConnection extracts the remote RTP address and port from SDP
func parseSDPConnection(sdpBody []byte) (*net.UDPAddr, error) {
	parsed := &sdp.SessionDescription{}
	if err := parsed.Unmarshal(sdpBody); err != nil {
		return nil, fmt.Errorf("unmarshal SDP: %w", err)
	}

	// Get connection address from session level
	var ip string
	if parsed.ConnectionInformation != nil && parsed.ConnectionInformation.Address != nil {
		ip = parsed.ConnectionInformation.Address.Address
	}

	// Get port from first media description
	var port int
	if len(parsed.MediaDescriptions) > 0 {
		md := parsed.MediaDescriptions[0]
		port = md.MediaName.Port.Value

		// Check media-level connection info if session-level is missing
		if ip == "" && md.ConnectionInformation != nil && md.ConnectionInformation.Address != nil {
			ip = md.ConnectionInformation.Address.Address
		}
	}

	if ip == "" || port == 0 {
		return nil, fmt.Errorf("no connection info in SDP")
	}

	// Strip any multicast TTL/count suffixes
	if idx := strings.Index(ip, "/"); idx != -1 {
		ip = ip[:idx]
	}

	return &net.UDPAddr{IP: net.ParseIP(ip), Port: port}, nil
}

// threadSafeWriter wraps gorilla/websocket with a mutex for thread-safe writes
type threadSafeWriter struct {
	*websocket.Conn
	sync.Mutex
}

func (t *threadSafeWriter) WriteJSON(v interface{}) error {
	t.Lock()
	defer t.Unlock()
	return t.Conn.WriteJSON(v)
}
