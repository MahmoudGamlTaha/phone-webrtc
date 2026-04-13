import { getToken } from './api';

let ws = null;
let pc = null;
let localStream = null;
let onEvent = null;
let callActive = false;
let iceCandidateQueue = [];

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

  // Always connect WebSocket to the Go backend on port 8080
  // (Vite dev server runs on a different port and doesn't proxy WS well)
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsHost = '13.53.39.195:8080';
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

  // Create WebRTC PeerConnection (PCMU only)
  pc = new RTCPeerConnection({
    iceServers: [{ urls: 'stun:stun.l.google.com:19302' }],
    codecPreferences: ['audio/PCMU'],
  });

  // Get microphone
  const hasMic = !!(navigator.mediaDevices && navigator.mediaDevices.getUserMedia);
  if (hasMic) {
    navigator.mediaDevices.getUserMedia({ audio: true, video: false })
      .then(stream => {
        localStream = stream;
        stream.getTracks().forEach(track => pc.addTrack(track, stream));
        emit('mic-ready', '');
        createOffer();
      })
      .catch(err => {
        emit('mic-error', err.message);
        pc.addTransceiver('audio', { direction: 'recvonly' });
        createOffer();
      });
  } else {
    pc.addTransceiver('audio', { direction: 'recvonly' });
    createOffer();
  }

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

async function createOffer() {
  try {
    const offer = await pc.createOffer();
    await pc.setLocalDescription(offer);
    sendWS('offer', JSON.stringify(pc.localDescription));
  } catch (e) {
    emit('offer-error', e.message);
  }
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
      pc.setRemoteDescription(JSON.parse(msg.data))
        .then(() => {
          // Drain buffered ICE candidates
          return drainIceCandidates();
        })
        .then(() => pc.createAnswer())
        .then(answer => pc.setLocalDescription(answer))
        .then(() => sendWS('answer', JSON.stringify(pc.localDescription)))
        .catch(e => emit('answer-error', e.message));
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
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({ event, data }));
  }
}

export function dial(extension, customerID) {
  const data = customerID ? `${extension}:${customerID}` : extension;
  callActive = true;
  emit('dialing', extension);
  sendWS('dial', data);
}

export function hangup() {
  sendWS('hangup', '');
  callActive = false;
  if (localStream) {
    localStream.getTracks().forEach(t => t.stop());
    localStream = null;
  }
}

export function isCallActive() {
  return callActive;
}

export function disconnect() {
  hangup();
  if (pc) { pc.close(); pc = null; }
  if (ws) {
    // Only close if actually open, avoid closing during CONNECTING
    if (ws.readyState === WebSocket.OPEN) {
      ws.close();
    }
    ws = null;
  }
  iceCandidateQueue = [];
}
