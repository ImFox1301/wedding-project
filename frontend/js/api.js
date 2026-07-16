const API_BASE = '/api';

function getToken() {
  // A tab-scoped token (sessionStorage) takes precedence over the shared
  // localStorage one. Used by admin link preview so the guest token used for
  // read-only calls stays confined to the preview tab and never clobbers the
  // admin session in other tabs.
  return sessionStorage.getItem('token') || localStorage.getItem('token');
}

function setAuth(token, role, guestId) {
  localStorage.setItem('token', token);
  localStorage.setItem('role', role);
  if (guestId) localStorage.setItem('guest_id', guestId);
}

function clearAuth() {
  localStorage.removeItem('token');
  localStorage.removeItem('role');
  localStorage.removeItem('guest_id');
}

function getRole() {
  return localStorage.getItem('role');
}

async function apiRequest(method, path, body, isFormData) {
  const token = getToken();
  const headers = {};
  if (token) headers['Authorization'] = 'Bearer ' + token;
  if (!isFormData && body) headers['Content-Type'] = 'application/json';

  const opts = { method, headers };
  if (body) {
    opts.body = isFormData ? body : JSON.stringify(body);
  }

  const res = await fetch(API_BASE + path, opts);
  if (res.status === 401) {
    clearAuth();
    window.location.href = '/login.html';
    return;
  }

  let data;
  try { data = await res.json(); } catch { data = {}; }

  if (!res.ok) {
    throw new Error(data.error || 'Request failed');
  }
  return data;
}

const api = {
  get:    (path)        => apiRequest('GET', path),
  post:   (path, body)  => apiRequest('POST', path, body),
  put:    (path, body)  => apiRequest('PUT', path, body),
  delete: (path)        => apiRequest('DELETE', path),
  upload: (path, form)  => apiRequest('POST', path, form, true),

  // Auth
  login: (login, password) => api.post('/auth/login', { login, password }),
  me:    ()                => api.get('/auth/me'),

  // Admin - guests
  guests: {
    list:   ()         => api.get('/admin/guests'),
    get:    (id)       => api.get('/admin/guests/' + id),
    create: (data)     => api.post('/admin/guests', data),
    update: (id, data) => api.put('/admin/guests/' + id, data),
    delete: (id)       => api.delete('/admin/guests/' + id),
  },

  // Admin - groups
  groups: {
    list:   ()         => api.get('/admin/groups'),
    get:    (id)       => api.get('/admin/groups/' + id),
    create: (data)     => api.post('/admin/groups', data),
    update: (id, data) => api.put('/admin/groups/' + id, data),
    delete: (id)       => api.delete('/admin/groups/' + id),
  },

  // Admin - links
  links: {
    list:      ()         => api.get('/admin/links'),
    available: ()         => api.get('/admin/links/available'),
    create:    (data)     => api.post('/admin/links', data),
    delete:    (id)       => api.delete('/admin/links/' + id),
  },

  // Admin - gifts
  gifts: {
    list:        (role)     => api.get('/admin/gifts' + (role ? '?role=' + role : '')),
    create:      (data)     => api.post('/admin/gifts', data),
    update:      (id, data) => api.put('/admin/gifts/' + id, data),
    delete:      (id)       => api.delete('/admin/gifts/' + id),
    uploadPhoto: (id, form) => apiRequest('POST', '/admin/gifts/' + id + '/photo', form, true),
    deletePhoto: (id)       => api.delete('/admin/gifts/' + id + '/photo'),
  },

  // Admin - sections
  sections: {
    list:   (role)     => api.get('/admin/sections' + (role ? '?role=' + role : '')),
    create: (data)     => api.post('/admin/sections', data),
    update: (id, data) => api.put('/admin/sections/' + id, data),
    order:  (id, ord)  => api.put('/admin/sections/' + id + '/order', { order: ord }),
    delete: (id)       => api.delete('/admin/sections/' + id),
    uploadPhoto: (id, form) => apiRequest('POST', '/admin/sections/' + id + '/photos', form, true),
    deletePhoto: (sId, pId) => api.delete('/admin/sections/' + sId + '/photos/' + pId),
  },

  // Admin - personal sections (per guest / per group)
  personalSections: {
    list:   ()     => api.get('/admin/personal-sections'),
    create: (data) => api.post('/admin/personal-sections', data),
  },

  // Admin - drinks (preferred drinks list)
  drinks: {
    list:     ()         => api.get('/admin/drinks'),
    create:   (data)     => api.post('/admin/drinks', data),
    update:   (id, data) => api.put('/admin/drinks/' + id, data),
    delete:   (id)       => api.delete('/admin/drinks/' + id),
    comments: ()         => api.get('/admin/drink-comments'),
  },

  // Admin - settings
  settings: {
    get:    ()         => api.get('/admin/settings'),
    update: (data)     => api.put('/admin/settings', data),
  },

  // Admin - page subtitles (stored in settings)
  pageSubtitle: {
    get: ()        => api.get('/admin/settings'),
    save: (friends, family) => api.put('/admin/settings', {
      page_subtitle_friends: friends,
      page_subtitle_family:  family,
    }),
  },

  // Admin - music
  music: {
    list:   (role)     => api.get('/admin/music' + (role ? '?role=' + role : '')),
    upload: (form)     => apiRequest('POST', '/admin/music', form, true),
    delete: (id)       => api.delete('/admin/music/' + id),
    order:  (id, ord)  => api.put('/admin/music/' + id + '/order', { order: ord }),
  },

  // Admin - stats
  stats: {
    visits:     () => api.get('/admin/stats/visits'),
    cottage:    () => api.get('/admin/stats/cottage'),
    tournament: () => api.get('/admin/stats/tournament'),
    loft:       () => api.get('/admin/stats/loft'),
    attendance: () => api.get('/admin/stats/attendance'),
    drinks:     () => api.get('/admin/stats/drinks'),
  },

  // Admin - comments
  comments: {
    list:  ()                => api.get('/admin/comments'),
    reply: (guestId, reply)  => api.put('/admin/comments/' + guestId + '/reply', { reply }),
  },

  // Chat
  chat: {
    messages:      ()     => api.get('/guest/chat/messages'),
    unread:        ()     => api.get('/guest/chat/unread'),
    seen:          ()     => api.post('/guest/chat/seen'),
    adminMessages: (room) => api.get('/admin/chat/messages?room=' + room),
    clear:         (room) => api.delete('/admin/chat/messages?room=' + room),
    deleteMessage: (id)   => api.delete('/admin/chat/messages/' + id),
  },

  // Guest
  invite: (token) => fetch(API_BASE + '/invite/' + token).then(r => r.json()),
  // Admin-only preview: sends admin auth, records no visit, issues no interaction rights.
  invitePreview: (token) => apiRequest('GET', '/invite/' + token + '?preview=1'),

  guestMe:      ()           => api.get('/guest/me'),
  saveFriend:   (data)       => api.put('/guest/response/friend', data),
  saveFamily:   (data)       => api.put('/guest/response/family', data),
  saveAttendance: (data)     => api.put('/guest/response/attendance', data),
  guestGifts:  ()           => api.get('/guest/gifts'),
  pickGift:    (id)         => api.post('/guest/gifts/' + id + '/pick'),
  unpickGift:  (id)         => apiRequest('DELETE', '/guest/gifts/' + id + '/pick'),
  saveGroupGiftPick: (data) => api.put('/guest/group-gift-pick', data),
  groupGiftPicks:    ()     => api.get('/guest/group-gift-picks'),
  guestMusic:  ()           => api.get('/guest/music'),
  friendsList: (all)        => api.get('/guest/friends' + (all ? '?all=1' : '')),
};

// Build the chat WebSocket URL (token passed as query — browsers can't set headers on WS)
function chatWsUrl(room) {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  let u = proto + '://' + location.host + '/api/ws/chat?token=' + encodeURIComponent(getToken() || '');
  if (room) u += '&room=' + room;
  return u;
}

// Require admin auth
function requireAdmin() {
  const role = getRole();
  if (!getToken() || role !== 'admin') {
    window.location.href = '/login.html';
  }
}

// Require guest auth
function requireGuest() {
  if (!getToken() || getRole() === 'admin') {
    window.location.href = '/login.html';
  }
}

// Show alert
function showAlert(container, msg, type = 'success') {
  const el = document.createElement('div');
  el.className = 'alert alert-' + type;
  el.textContent = msg;
  container.prepend(el);
  setTimeout(() => el.remove(), 4000);
}

// Confirm dialog
function confirmDialog(msg) {
  return window.confirm(msg);
}
