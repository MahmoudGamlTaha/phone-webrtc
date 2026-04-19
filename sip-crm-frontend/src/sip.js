import { getToken } from './api';

let ws = null;
let pc = null;
let localStream = null;
let onEvent = null;
let callActive = false;
let iceCandidateQueue = [];
let micPromise = null; // Deduplicates getUserMedia calls

export function setEventHandler(handler) {
  onEvent = handler;
}

function emit(event, data) {
  if (onEvent) onEvent(event, data);
}

export function connect() {
  // Idempotent: skip if already connected or connecting
  if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
    return;
  }

  // Use same host as the page (works for both HTTP and HTTPS)
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsHost = window.location.host;
  const wsUrl = `${protocol}//${wsHost}/ws`;

  ws = new WebSocket(wsUrl);
  const currentWS = ws; // capture to avoid null race with disconnect()

  ws.onopen = () => {
    emit('ws-connected', '');
    // Authenticate WebSocket with our login token
    const token = getToken();
    if (token) {
      currentWS.send(JSON.stringify({ event: 'auth', data: token }));
    }
  };

  ws.onclose = () => {
    emit('ws-disconnected', '');
    ws = null;
  };

  ws.onerror = (err) => {
    emit('ws-error', err.message || 'WebSocket error');
  };

  ws.onmessage = (evt) => {
    try {
      const msg = JSON.parse(evt.data);
      handleSignaling(msg);
    } catch (e) {
      console.error('WS parse error:', e);
    }
  };

  // Create WebRTC PeerConnection - backend is the offerer, we are the answerer
  pc = new RTCPeerConnection({
    iceServers: [{ urls: 'stun:stun.l.google.com:19302' }],
  });

  // Do NOT addTransceiver here - ensureMicStream uses addTrack instead, which
  // creates a transceiver with our mic track already attached. When the backend's
  // offer arrives, setRemoteDescription matches this transceiver to the m-line,
  // and createAnswer includes our mic SSRC. Pre-creating with addTransceiver
  // (no track) caused m-line mismatch → one-way audio.

  // Request mic early so it's ready when the offer arrives
  ensureMicStream();

  pc.onicecandidate = (e) => {
    if (e.candidate) {
      sendWS('candidate', JSON.stringify(e.candidate.toJSON()));
    }
  };

  pc.ontrack = (e) => {
    const remoteAudio = document.getElementById('remote-audio');
    if (remoteAudio && e.streams[0]) {
      remoteAudio.srcObject = e.streams[0];
      remoteAudio.play().catch(() => {});
    }
    emit('remote-audio', '');
  };

  pc.onconnectionstatechange = () => {
    emit('pc-state', pc.connectionState);
  };
}

// ensureMicStream requests microphone and adds the track to the PeerConnection.
// Must be called BEFORE setRemoteDescription so the track is on the transceiver
// when createAnswer runs. This is the standard WebRTC answerer pattern.
async function ensureMicStream() {
  if (localStream) return true;
  if (micPromise) return micPromise;
  micPromise = (async () => {
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true, video: false });
      localStream = stream;
      const track = stream.getAudioTracks()[0];
      if (track && pc) {
        // addTrack creates an audio transceiver with our mic track.
        // When setRemoteDescription processes the offer, it will match this
        // transceiver to the offer's audio m-line by kind.
        pc.addTrack(track, stream);
        console.log('ensureMicStream: mic track added via addTrack, track id=', track.id);
      }
      emit('mic-ready', '');
      return true;
    } catch (err) {
      emit('mic-error', err.message);
      micPromise = null;
      return false;
    }
  })();
  return micPromise;
}

async function drainIceCandidates() {
  while (iceCandidateQueue.length > 0) {
    const candidate = iceCandidateQueue.shift();
    try {
      await pc.addIceCandidate(candidate);
    } catch (e) {
      console.error('Drain ICE candidate error:', e);
    }
  }
}

function handleSignaling(msg) {
  switch (msg.event) {
    case 'offer':
      // Standard WebRTC answerer pattern:
      // 1) ensureMicStream - adds mic track via addTrack BEFORE setRemoteDescription
      // 2) setRemoteDescription - matches our audio transceiver to offer's m-line
      // 3) createAnswer - includes our mic SSRC in the answer
      ensureMicStream()
        .then(() => {
          console.log('offer handler: mic ready, transceivers=', pc.getTransceivers().length,
            pc.getTransceivers().map(t => ({ dir: t.direction, track: t.sender?.track?.kind })));
          return pc.setRemoteDescription(JSON.parse(msg.data));
        })
        .then(() => drainIceCandidates())
        .then(() => pc.createAnswer())
        .then(answer => {
          const hasSSRC = answer.sdp && answer.sdp.includes('ssrc');
          console.log('createAnswer: hasSSRC=', hasSSRC,
            'transceivers=', pc.getTransceivers().map(t => ({ dir: t.direction, track: t.sender?.track?.kind })));
          if (!hasSSRC) {
            console.error('WARNING: answer SDP has no SSRC - mic audio will NOT be sent!');
          }
          return pc.setLocalDescription(answer);
        })
        .then(() => {
          console.log('SDP answer sent, type:', pc.localDescription.type);
          sendWS('answer', JSON.stringify(pc.localDescription));
        })
        .catch(e => {
          console.error('offer handling error:', e);
          emit('answer-error', e.message);
        });
      break;
    case 'answer':
      pc.setRemoteDescription(JSON.parse(msg.data))
        .then(() => drainIceCandidates())
        .catch(e => emit('sdp-error', e.message));
      break;
    case 'candidate':
      if (pc.remoteDescription) {
        pc.addIceCandidate(JSON.parse(msg.data))
          .catch(e => console.error('ICE candidate error:', e));
      } else {
        // Buffer until remote description is set
        iceCandidateQueue.push(JSON.parse(msg.data));
      }
      break;
    case 'auth-ok':
      emit('auth-ok', msg.data);
      break;
    case 'auth-error':
      emit('auth-error', msg.data);
      break;
    case 'call-started':
      callActive = true;
      emit('call-started', msg.data);
      break;
    case 'call-ended':
      callActive = false;
      emit('call-ended', msg.data);
      break;
    case 'dial-error':
      callActive = false;
      emit('dial-error', msg.data);
      break;
    default:
      emit(msg.event, msg.data);
  }
}

function sendWS(event, data) {
  if (!ws) {
    console.error(`sendWS('${event}'): WebSocket is null - not connected`);
    return;
  }
  if (ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({ event, data }));
  } else if (ws.readyState === WebSocket.CONNECTING) {
    console.warn(`sendWS('${event}'): WebSocket still connecting, retrying in 500ms`);
    setTimeout(() => sendWS(event, data), 500);
  } else {
    console.error(`sendWS('${event}'): WebSocket closed (state=${ws.readyState}), reconnecting`);
    connect();
    setTimeout(() => sendWS(event, data), 1000);
  }
}

export async function dial(extension, customerID) {
  const data = customerID ? `${extension}:${customerID}` : extension;
  callActive = true;
  emit('dialing', extension);
  // Ensure mic is available before dialing
  await ensureMicStream();
  console.log(`sip.dial: sending dial event for ${extension}, ws state=${ws ? ws.readyState : 'null'}`);
  sendWS('dial', data);
}

export function hangup() {
  sendWS('hangup', '');
  callActive = false;
  // Don't stop localStream on hangup - keep mic for next call
  // Stream will be stopped on disconnect()
}

export function isCallActive() {
  return callActive;
}

export function disconnect() {
  hangup();
  if (localStream) {
    localStream.getTracks().forEach(t => t.stop());
    localStream = null;
  }
  if (pc) { pc.close(); pc = null; }
  if (ws) {
    // Only close if actually open, avoid closing during CONNECTING
    if (ws.readyState === WebSocket.OPEN) {
      ws.close();
    }
    ws = null;
  }
  micPromise = null;
  iceCandidateQueue = [];
}
