const API_BASE = '/api';

let token = localStorage.getItem('token') || '';

export function setToken(t) {
  token = t;
  localStorage.setItem('token', t);
}

export function getToken() {
  return token;
}

export function clearToken() {
  token = '';
  localStorage.removeItem('token');
}

async function api(path, options = {}) {
  const headers = { 'Content-Type': 'application/json', ...options.headers };
  if (token) headers['Authorization'] = `Bearer ${token}`;
  const res = await fetch(`${API_BASE}${path}`, { ...options, headers });
  if (res.status === 401) {
    clearToken();
    window.location.reload();
    throw new Error('Unauthorized');
  }
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || 'Request failed');
  return data;
}

export const auth = {
  login: (username, password) =>
    api('/login', { method: 'POST', body: JSON.stringify({ username, password }) }),
  register: (data) =>
    api('/register', { method: 'POST', body: JSON.stringify(data) }),
  profile: () => api('/profile'),
};

export const customers = {
  list: () => api('/customers'),
  get: (id) => api(`/customers/detail?id=${id}`),
  create: (data) => api('/customers', { method: 'POST', body: JSON.stringify(data) }),
  update: (data) => api('/customers/detail', { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id) => api(`/customers/detail?id=${id}`, { method: 'DELETE' }),
};

export const calls = {
  list: (limit = 50) => api(`/calls?limit=${limit}`),
  recent: (limit = 20) => api(`/calls/recent?limit=${limit}`),
};

export const tasks = {
  list: (status = '') => api(`/tasks${status ? `?status=${status}` : ''}`),
  create: (data) => api('/tasks', { method: 'POST', body: JSON.stringify(data) }),
  update: (data) => api('/tasks/detail', { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id) => api(`/tasks/detail?id=${id}`, { method: 'DELETE' }),
};

export const sipConfig = {
  get: () => api('/sip-config'),
  update: (data) => api('/sip-config', { method: 'PUT', body: JSON.stringify(data) }),
};

export const dashboard = {
  stats: () => api('/dashboard'),
};
