# sip-webrtc-gateway

A SIP-to-WebRTC gateway that bridges SIP calls between a PBX and WebRTC browser clients.

Supports both **inbound** (SIP phone calls in) and **outbound** (browser dials SIP extension) with **bidirectional audio**.

## Architecture

```
Browser (WebRTC) <--WebSocket--> Gateway <--SIP/UDP--> PBX
```

## Outbound Flow (browser dials SIP extension)

1. Browser opens WebSocket, establishes WebRTC PeerConnection (audio sendrecv)
2. Browser sends "dial" event with target extension
3. Gateway sends SIP INVITE to PBX (with digest auth if challenged)
4. PBX answers, RTP flows bidirectionally:
   - Browser mic → WebRTC track → RTP → PBX
   - PBX → RTP → WebRTC track → Browser speaker

## Inbound Flow (SIP phone calls in)

1. SIP INVITE arrives from PBX on UDP 5060
2. Gateway answers, creates audio track for WebRTC peers
3. RTP audio from SIP is forwarded to all connected browsers

## Requirements

- Go 1.24+
- A SIP PBX (for outbound calls, the PBX must support SIP REGISTER + digest auth)
- The gateway must be reachable on both SIP (UDP 5060) and HTTP (TCP 8080) ports

## Run

```bash
cd sip-webrtc-gateway
go run main.go [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-http-port` | 8080 | HTTP/WebSocket server port |
| `-sip-listen-port` | 5060 | SIP listener port for inbound calls |
| `-sip-server` | | SIP server address (host:port) for outbound calls |
| `-sip-username` | | SIP extension/username to register as |
| `-sip-password` | | SIP password for digest auth |
| `-sip-domain` | | SIP domain (defaults to sip-server host) |
| `-public-ip` | auto | Public IP for SDP (auto-detected if empty) |

### Example — Outbound calls to PBX

```bash
go run main.go \
  -public-ip YOUR_PUBLIC_IP \
  -sip-server 173.199.70.125:5668 \
  -sip-username 7029 \
  -sip-password 70e4bc50f5966ed199593c5ce828b9ed \
  -sip-domain 173.199.70.125
```

### Example — Inbound calls only

```bash
go run main.go -public-ip YOUR_PUBLIC_IP
```

## Usage

1. Start the gateway with your PBX credentials
2. Open `http://<gateway-ip>:8080` in your browser
3. Click **Connect** — the browser establishes a WebRTC PeerConnection (requests microphone)
4. Type an extension (e.g. `7030`) and click **Call** — the gateway sends a SIP INVITE to the PBX
5. Audio flows bidirectionally: browser mic → PBX, PBX → browser speaker
6. Click **Hangup** to end the call

## Codec Support

Currently supports **PCMU (G.711 μ-law)** at 8000Hz — the most common codec for SIP/PBX systems.

To add Opus or other codecs, modify the `generateSDPAnswer` / `buildSDPOffer` functions and the WebRTC `TrackLocalStaticRTP` codec capability.

## Notes

- The gateway registers with the PBX using SIP REGISTER with digest authentication.
- Inbound calls are accepted without authentication. For production use, add SIP auth on the server side.
- Each browser connection can make one outbound SIP call at a time.
- Inbound calls are broadcast to all connected browsers.
