const API_BASE = '/api';

function getToken() {
  return localStorage.getItem('token');
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
  },

  // Admin - comments
  comments: {
    list:  ()                => api.get('/admin/comments'),
    reply: (guestId, reply)  => api.put('/admin/comments/' + guestId + '/reply', { reply }),
  },

  // Guest
  invite: (token) => fetch(API_BASE + '/invite/' + token).then(r => r.json()),

  guestMe:     ()           => api.get('/guest/me'),
  saveFriend:  (data)       => api.put('/guest/response/friend', data),
  saveFamily:  (data)       => api.put('/guest/response/family', data),
  guestGifts:  ()           => api.get('/guest/gifts'),
  pickGift:    (id)         => api.post('/guest/gifts/' + id + '/pick'),
  unpickGift:  (id)         => apiRequest('DELETE', '/guest/gifts/' + id + '/pick'),
  guestMusic:  ()           => api.get('/guest/music'),
  friendsList: ()           => api.get('/guest/friends'),
};

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
