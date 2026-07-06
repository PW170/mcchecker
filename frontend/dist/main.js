let running = false;

function $(id) { return document.getElementById(id); }

// Tab switching
document.querySelectorAll('.tab-btn').forEach(function(btn) {
  btn.addEventListener('click', function() {
    document.querySelectorAll('.tab-btn').forEach(function(b) { b.classList.remove('active'); });
    document.querySelectorAll('.tab-content').forEach(function(t) { t.classList.remove('active'); });
    btn.classList.add('active');
    $('tab-' + btn.dataset.tab).classList.add('active');
  });
});

// Config form
function buildConfigForm(config) {
  var html = '';
  var fields = [
    { key: 'CookieCheck', label: 'Cookie Check', desc: 'Check cookie files from the cookies folder', type: 'bool' },
    { key: 'CookiePath', label: 'Cookie Path', desc: 'Path to cookies folder', type: 'text' },
    { key: 'HypixelCheck', label: 'Hypixel API Check', desc: 'Check Hypixel stats via API (requires API key)', type: 'bool' },
    { key: 'HypixelBan', label: 'Hypixel Ban Check', desc: 'Check if account is banned on Hypixel', type: 'bool' },
    { key: 'HypixelAPIKey', label: 'Hypixel API Key', desc: 'API key for Hypixel stats', type: 'text' },
    { key: 'MSRewards', label: 'MS Rewards', desc: 'Check Microsoft Rewards points', type: 'bool' },
    { key: 'GamepassPC', label: 'Game Pass PC', desc: 'Check for Game Pass PC entitlement', type: 'bool' },
    { key: 'GamepassUltimate', label: 'Game Pass Ultimate', desc: 'Check for Game Pass Ultimate', type: 'bool' },
    { key: 'XboxPerks', label: 'Xbox Perks', desc: 'Check Xbox Game Pass perks', type: 'bool' },
    { key: 'NitroPromo', label: 'Nitro Promo', desc: 'Scan for Discord Nitro promo links', type: 'bool' },
    { key: 'ProxyMode', label: 'Proxy Mode', desc: 'Proxy mode (none, http, socks5)', type: 'text' },
    { key: 'RetryRateLimited', label: 'Retry Rate Limited', desc: 'Retry accounts hit by rate limiting', type: 'bool' },
    { key: 'Webhook', label: 'Webhook URL', desc: 'Discord webhook for hits', type: 'text' },
    { key: 'DefaultWebhook', label: 'Default Webhook', desc: 'Fallback webhook if none set', type: 'text' },
  ];

  fields.forEach(function(f) {
    var val = config[f.key];
    if (val === undefined) val = false;
    if (f.type === 'bool') {
      var active = val ? ' active' : '';
      html += '<div class="config-row" data-key="' + f.key + '" data-type="bool">' +
        '<div><label>' + f.label + '</label><div class="config-desc">' + f.desc + '</div></div>' +
        '<div class="config-toggle' + active + '"></div></div>';
    } else {
      html += '<div class="config-row" data-key="' + f.key + '" data-type="text">' +
        '<div><label>' + f.label + '</label><div class="config-desc">' + f.desc + '</div></div>' +
        '<input class="config-input" type="text" value="' + (val || '') + '"></div>';
    }
  });

  $('config-form').innerHTML = html;

  document.querySelectorAll('.config-toggle').forEach(function(toggle) {
    toggle.addEventListener('click', function() {
      this.classList.toggle('active');
    });
  });
}

function readConfigFromForm() {
  var cfg = {};
  document.querySelectorAll('.config-row').forEach(function(row) {
    var key = row.dataset.key;
    var type = row.dataset.type;
    if (type === 'bool') {
      cfg[key] = row.querySelector('.config-toggle').classList.contains('active');
    } else {
      cfg[key] = row.querySelector('.config-input').value;
    }
  });
  return cfg;
}

// Logging
function addLog(level, msg) {
  var area = $('log-area');
  var div = document.createElement('div');
  div.className = 'log-entry';
  var cls = 'log-info';
  if (level === 1) cls = 'log-success';
  else if (level === 2) cls = 'log-error';
  else if (level === 3) cls = 'log-hypixel';
  div.classList.add(cls);
  div.textContent = msg;
  area.appendChild(div);
  area.scrollTop = area.scrollHeight;
}

function updateStats(s) {
  $('stat-mc').textContent = s.mcHits || 0;
  $('stat-xgpu').textContent = s.xgpuHits || 0;
  $('stat-rp').textContent = s.rpHits || 0;
  $('stat-valid').textContent = s.validCount || 0;
  $('stat-hbanned').textContent = s.hypixelBanned || 0;
  $('stat-hunban').textContent = s.hypixelUnban || 0;
  $('stat-cvalid').textContent = s.cookieValid || 0;
  $('stat-cinvalid').textContent = s.cookieInvalid || 0;
  $('stat-cpm').textContent = Math.round(s.cpm || 0);
  var secs = Math.floor(s.elapsedSeconds || 0);
  var m = Math.floor(secs / 60);
  var sec = secs % 60;
  $('stat-time').textContent = m + ':' + (sec < 10 ? '0' : '') + sec;
}

function showResults(s) {
  $('res-checked').textContent = s.totalChecked || 0;
  $('res-mc').textContent = s.mcHits || 0;
  $('res-xgpu').textContent = s.xgpuHits || 0;
  $('res-rp').textContent = s.rpHits || 0;
  $('res-hbanned').textContent = s.hypixelBanned || 0;
  $('res-hunban').textContent = s.hypixelUnban || 0;
  $('res-cvalid').textContent = s.cookieValid || 0;
  $('res-cinvalid').textContent = s.cookieInvalid || 0;
  $('res-cpm').textContent = Math.round(s.cpm || 0);
  var secs = Math.floor(s.elapsedSeconds || 0);
  var m = Math.floor(secs / 60);
  var sec = secs % 60;
  $('res-time').textContent = m + ':' + (sec < 10 ? '0' : '') + sec;
}

// Wails event handlers
if (window.wails && window.wails.runtime) {
  window.runtime = window.wails.runtime;
}

if (window.runtime) {
  window.runtime.EventsOn('checker:log', function(data) {
    addLog(data.level, data.msg);
  });

  window.runtime.EventsOn('checker:progress', function(s) {
    updateStats(s);
  });

  window.runtime.EventsOn('checker:complete', function(s) {
    running = false;
    $('btn-start').disabled = false;
    $('btn-stop').disabled = true;
    $('btn-start').textContent = '\u25B6 Start';
    $('bottom-status').textContent = 'Check complete.';
    if (s.error) {
      $('bottom-status').textContent = 'Error: ' + s.error;
    } else {
      showResults(s);
    }
  });

  window.runtime.EventsOn('checker:rundir', function(dir) {
    $('run-dir').textContent = '\u{1F4C1} ' + dir;
  });
}

// Buttons
$('btn-start').addEventListener('click', function() {
  if (running) return;
  running = true;
  $('btn-start').disabled = true;
  $('btn-stop').disabled = false;
  $('btn-start').textContent = '\u25B6 Running...';
  $('log-area').innerHTML = '';
  $('bottom-status').textContent = 'Checking...';

  var stats = ['stat-mc','stat-xgpu','stat-rp','stat-valid','stat-hbanned','stat-hunban','stat-cvalid','stat-cinvalid','stat-cpm','stat-time'];
  stats.forEach(function(id) { $(id).textContent = '0'; });

  var threads = parseInt($('threads').value) || 50;
  window.go.main.App.StartChecking(threads);
});

$('btn-stop').addEventListener('click', function() {
  window.go.main.App.StopChecking();
  $('bottom-status').textContent = 'Stopping...';
});

$('btn-save-config').addEventListener('click', function() {
  var cfg = readConfigFromForm();
  var json = JSON.stringify(cfg, null, 2);
  window.go.main.App.SaveConfig(json).then(function(result) {
    var status = $('config-status');
    if (result === 'ok') {
      status.className = 'config-status config-ok';
      status.textContent = 'Saved!';
    } else {
      status.className = 'config-status config-err';
      status.textContent = result;
    }
    setTimeout(function() { status.textContent = ''; }, 3000);
  });
});

$('btn-open-run').addEventListener('click', function() {
  window.go.main.App.GetRunDir().then(function(dir) {
    if (dir) window.go.main.App.OpenFolder(dir);
  });
});

$('btn-open-results').addEventListener('click', function() {
  window.go.main.App.OpenFolder('results');
});

// Load initial config
document.addEventListener('DOMContentLoaded', function() {
  window.go.main.App.LoadConfig().then(function(json) {
    try {
      var cfg = JSON.parse(json);
      buildConfigForm(cfg);
    } catch(e) {
      console.error('Failed to parse config', e);
    }
  });
});

// Bottom status bar
$('bottom-status').textContent = 'Ready. Loaded ' + (new Date().toLocaleTimeString());
