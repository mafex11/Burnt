/* Burnt for Windows — popover dashboard.
 *
 * Vanilla JS, no build step, no network. Talks to Go over the WebView2 bridge
 * (see README.md for the full contract). Renders three pages into #root:
 * dashboard, settings, wrapped — and calls burnt_resize() after every paint so
 * the frameless window can size itself to the content.
 */
(function () {
  'use strict';

  /* ============================================================ constants */

  var MAX_HEIGHT = 720;                 // Go caps the window here; we scroll past it
  var MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun',
                'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

  var TOOL_COLOR = { claude: 'var(--claude)', codex: 'var(--codex)' };
  var TOOL_TINT = {
    claude: 'rgba(242,160,61,.18)',
    codex: 'rgba(61,184,104,.18)'
  };

  var MENU_MODES = [
    ['todayCost', 'Today $'],
    ['todayTokens', 'Today tokens'],
    ['weekCost', 'Week $'],
    ['iconOnly', 'Icon only']
  ];
  var STYLES = [['minimal', 'Minimal'], ['standard', 'Standard'], ['detailed', 'Detailed']];
  var STYLE_RANK = { minimal: 0, standard: 1, detailed: 2 };

  /* Heatmap ramp — ported verbatim from HeatmapView.swift (absolute $50 bands,
     capped at $1000, linearly interpolated in sRGB). */
  var HEAT_BAND = 50;
  var HEAT_MAX_BANDS = 20;
  var HEAT_RAMP = [
    [0.20, 0.45, 0.55], [0.24, 0.72, 0.49], [0.55, 0.78, 0.30], [0.86, 0.78, 0.25],
    [0.95, 0.62, 0.24], [0.93, 0.45, 0.16], [0.86, 0.27, 0.10], [0.65, 0.13, 0.06]
  ];

  /* Inline SVG only — no icon font, no network. */
  var SVG = {
    gear: '<path d="M7 .8 6.8 2.4a4.7 4.7 0 0 0-1.1.45L4.4 1.9 2.5 3.8l.95 1.3a4.7 4.7 0 0 0-.46 1.1L1.4 6.4v2.7l1.6.2q.16.58.46 1.1l-.95 1.3 1.9 1.9 1.3-.95q.52.3 1.1.46l.2 1.6h2.7l.2-1.6q.58-.16 1.1-.46l1.3.95 1.9-1.9-.95-1.3q.3-.52.46-1.1l1.6-.2V6.4l-1.6-.2a4.7 4.7 0 0 0-.46-1.1l.95-1.3-1.9-1.9-1.3.95a4.7 4.7 0 0 0-1.1-.46L9.2.8Z" fill="none" stroke="currentColor" stroke-width="1.2" stroke-linejoin="round"/><circle cx="8" cy="7.75" r="2.15" fill="none" stroke="currentColor" stroke-width="1.2"/>',
    refresh: '<path d="M13.2 8a5.2 5.2 0 1 1-1.6-3.75" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/><path d="M13.4 1.9v3.1h-3.1" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>',
    back: '<path d="M10 2.5 4.5 8l5.5 5.5" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>',
    arrowUp: '<path d="M3.5 12.5 12.5 3.5M6 3.5h6.5V10" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>',
    arrowDown: '<path d="M3.5 3.5 12.5 12.5M12.5 6v6.5H6" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>'
  };

  function svg(name, cls) {
    return '<svg class="' + (cls || 'icon') + '" viewBox="0 0 16 16" aria-hidden="true">' +
      SVG[name] + '</svg>';
  }

  /* =========================================================== formatters */

  /* Formatters.cost — "$4.20"; >= 1000 drops cents and groups thousands. */
  function fcost(c) {
    c = Number(c) || 0;
    if (c >= 1000) {
      return '$' + Math.round(c).toString().replace(/\B(?=(\d{3})+(?!\d))/g, ',');
    }
    return '$' + c.toFixed(2);
  }

  function trimNum(x) { return x >= 100 ? String(Math.round(x)) : x.toFixed(1); }

  /* Formatters.tokens — 1_234_567 -> "1.2M". */
  function ftokens(n) {
    n = Number(n) || 0;
    if (n >= 1e9) return trimNum(n / 1e9) + 'B';
    if (n >= 1e6) return trimNum(n / 1e6) + 'M';
    if (n >= 1e3) return trimNum(n / 1e3) + 'K';
    return String(Math.round(n));
  }

  /* Formatters.percent — magnitude as a whole percent. */
  function fpercent(f) { return Math.round(Math.abs(Number(f) || 0) * 100) + '%'; }

  /* "2026-06-08" -> "Jun 8". */
  function prettyDate(iso) {
    var p = String(iso || '').split('-');
    if (p.length !== 3) return String(iso || '');
    var m = parseInt(p[1], 10);
    return (m >= 1 && m <= 12 ? MONTHS[m - 1] : '?') + ' ' + parseInt(p[2], 10);
  }

  function relTime(iso) {
    var t = Date.parse(iso);
    if (isNaN(t)) return 'unknown';
    var s = Math.max(0, (Date.now() - t) / 1000);
    if (s < 45) return 'just now';
    if (s < 5400) return Math.max(1, Math.round(s / 60)) + 'm ago';
    if (s < 86400) return Math.round(s / 3600) + 'h ago';
    return Math.round(s / 86400) + 'd ago';
  }

  function esc(s) {
    return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  }

  function cap(s) {
    s = String(s || '');
    return s ? s.charAt(0).toUpperCase() + s.slice(1) : s;
  }

  /* ============================================================== heatmap */

  function rgbOf(c) {
    return 'rgb(' + Math.round(c[0] * 255) + ',' + Math.round(c[1] * 255) + ',' +
      Math.round(c[2] * 255) + ')';
  }

  function rampColor(t) {
    t = Math.min(Math.max(t, 0), 1);
    var scaled = t * (HEAT_RAMP.length - 1);
    var i = Math.floor(scaled);
    if (i >= HEAT_RAMP.length - 1) return rgbOf(HEAT_RAMP[HEAT_RAMP.length - 1]);
    var f = scaled - i, a = HEAT_RAMP[i], b = HEAT_RAMP[i + 1];
    return rgbOf([a[0] + (b[0] - a[0]) * f, a[1] + (b[1] - a[1]) * f, a[2] + (b[2] - a[2]) * f]);
  }

  function heatColor(cost) {
    if (!(cost > 0)) return 'var(--heat-empty)';
    var band = Math.min(cost / HEAT_BAND, HEAT_MAX_BANDS);
    return rampColor(band / HEAT_MAX_BANDS);
  }

  /* ================================================================ state */

  var state = null;
  var page = 'dashboard';               // dashboard | settings | wrapped
  var wrappedAllTime = false;
  var copiedAt = 0;

  var params = new URLSearchParams(location.search);
  if (params.get('theme')) document.documentElement.setAttribute('data-theme', params.get('theme'));
  if (params.get('page')) page = params.get('page');

  function settings() { return (state && state.settings) || {}; }
  function summary() { return (state && state.summary) || {}; }
  function atLeast(level) {
    return (STYLE_RANK[settings().dashboardStyle] || 0) >= STYLE_RANK[level];
  }

  /* Bridge fields are tolerated under both the spec name and the Swift name. */
  function sparkPoints() { var s = summary(); return s.sparkline || s.weekByDay || []; }
  function heatPoints() { var s = summary(); return s.heatmap || s.heatmapDays || []; }
  function modelName(m) { return m.model || m.modelName || ''; }
  function projectName(p) { return p.project || p.name || ''; }
  function sliceTokens(x) { return x.tokens != null ? x.tokens : (x.totalTokens || 0); }

  /* =============================================================== bridge */

  function call(name) {
    var args = Array.prototype.slice.call(arguments, 1);
    var fn = window[name];
    if (typeof fn !== 'function') return Promise.resolve(null);
    try { return Promise.resolve(fn.apply(null, args)); }
    catch (e) { return Promise.reject(e); }
  }

  function pushSettings() { return call('burnt_setSettings', JSON.stringify(settings())); }

  function patch(key, value) {
    if (!state) return;
    state.settings = state.settings || {};
    state.settings[key] = value;
    pushSettings();
  }

  window.burntOnState = function (json) {
    try { state = typeof json === 'string' ? JSON.parse(json) : json; }
    catch (e) { return; }
    render();
  };

  // Host hook: the tray menu's "Settings…" item jumps straight to a page.
  window.burntShowPage = function (name) {
    page = (name === 'settings' || name === 'wrapped') ? name : 'dashboard';
    render();
  };

  /* ============================================================ dashboard */

  function heroBlock() {
    var s = summary();
    var today = s.today || {};
    var h = '<div class="hero-row"><div><div class="hero-val">' +
      esc(fcost(today.cost)) + '</div><div class="hero-sub"><span class="caption">today</span>';
    if (s.weekTrend !== null && s.weekTrend !== undefined) {
      var up = Number(s.weekTrend) >= 0;
      h += '<span class="trend ' + (up ? 'up' : 'down') + '">' +
        svg(up ? 'arrowUp' : 'arrowDown') +
        '<span class="num">' + esc(fpercent(s.weekTrend)) + '</span> vs last week</span>';
    }
    h += '</div></div><div class="hero-actions">' +
      '<button class="iconbtn" data-act="settings" title="Settings" aria-label="Settings">' +
      svg('gear') + '</button>' +
      '<button class="iconbtn" data-act="refresh" title="Refresh" aria-label="Refresh">' +
      svg('refresh', 'icon' + (state && state.loading ? ' spin' : '')) + '</button>' +
      '</div></div>';
    return h;
  }

  function budgetBlock() {
    var budget = Number(settings().dailyBudget) || 0;
    if (budget <= 0) return '';
    var spent = Number((summary().today || {}).cost) || 0;
    var ratio = spent / budget;
    var color = ratio > 1 ? 'var(--bad)' : (ratio >= 0.8 ? 'var(--warn)' : 'var(--good)');
    var w = (Math.min(ratio, 1) * 100).toFixed(1);
    return '<div class="budget"><div class="track"><div class="fill" style="width:' + w +
      '%;background:' + color + '"></div></div><div class="lbl num">' +
      esc(fpercent(ratio)) + ' of ' + esc(fcost(budget)) + '</div></div>';
  }

  function statsBlock() {
    var s = summary();
    function cell(k, t) {
      return '<div class="stat"><div class="k">' + k + '</div><div class="v">' +
        esc(fcost((t || {}).cost)) + '</div></div>';
    }
    return '<div class="stats">' + cell('Week', s.week) + cell('Month', s.month) +
      cell('All-time', s.allTime) + '</div>';
  }

  function paceBlock() {
    if (!atLeast('detailed')) return '';
    var s = summary();
    var txt = 'avg ' + fcost(s.avgPerDay) + '/day';
    if (s.projectedToday !== null && s.projectedToday !== undefined) {
      txt += ' · pace ~' + fcost(s.projectedToday) + ' today';
    }
    return '<div class="pace num">' + esc(txt) + '</div>';
  }

  function sparkBlock() {
    var pts = sparkPoints();
    var max = 0.01;
    pts.forEach(function (p) { max = Math.max(max, Number(p.cost) || 0); });
    var bars = pts.map(function (p, i) {
      var c = Number(p.cost) || 0;
      var hpx = Math.max(4, (c / max) * 40);
      return '<div class="b' + (c === 0 ? ' zero' : '') + '" style="height:' + hpx.toFixed(1) +
        'px" data-hov="spark" data-i="' + i + '"></div>';
    }).join('');
    return '<div class="spark-wrap"><div class="caption" id="spark-cap">last 14 days</div>' +
      '<div class="spark" data-hovgroup="spark">' + bars + '</div></div>';
  }

  function heatBlock() {
    if (!atLeast('standard')) return '';
    var days = heatPoints();
    var cells = days.map(function (d, i) {
      return '<div class="c" style="background:' + heatColor(Number(d.cost) || 0) +
        '" data-hov="heat" data-i="' + i + '"></div>';
    }).join('');
    var ramp = '';
    for (var i = 0; i < 10; i++) ramp += '<i style="background:' + rampColor(i / 9) + '"></i>';
    return '<div class="section-row"><span class="section-h">Last 12 weeks</span>' +
      '<span class="caption" id="heat-cap"></span></div>' +
      '<div class="heat" data-hovgroup="heat">' + cells + '</div>' +
      '<div class="legend"><span class="lb">$0</span><span class="ramp">' + ramp +
      '</span><span class="lb">$1k+</span></div>';
  }

  function breakdownRow(color, tint, label, fraction, cost, tokens) {
    var w = (Math.min(Math.max(fraction, 0), 1) * 100).toFixed(1);
    return '<div class="brow"><span class="dot" style="background:' + color + '"></span>' +
      '<span class="name" title="' + esc(label) + '">' + esc(label) + '</span>' +
      '<span class="prop"><span class="track" style="background:' + tint +
      '"><span class="fill" style="width:' + w + '%;background:' + color +
      ';display:block;border-radius:2px"></span></span></span>' +
      '<span class="tok">' + esc(ftokens(tokens)) + '</span>' +
      '<span class="cost">' + esc(fcost(cost)) + '</span></div>';
  }

  function maxCost(list) {
    var m = 0.01;
    (list || []).forEach(function (x) { m = Math.max(m, Number(x.cost) || 0); });
    return m;
  }

  function byToolBlock() {
    if (!atLeast('standard')) return '';
    var list = summary().byTool || [];
    if (!list.length) return '';
    var max = maxCost(list);
    return '<div class="section-h">By tool</div>' + list.map(function (t) {
      var color = TOOL_COLOR[t.tool] || 'var(--muted)';
      var tint = TOOL_TINT[t.tool] || 'var(--chip)';
      return breakdownRow(color, tint, cap(t.tool), (Number(t.cost) || 0) / max,
        t.cost, sliceTokens(t));
    }).join('');
  }

  function byModelBlock() {
    if (!atLeast('detailed')) return '';
    var s = summary();
    var list = (s.byModel || []).slice(0, 5);
    var out = '';
    if (list.length) {
      var max = maxCost(s.byModel);
      out += '<div class="section-h">By model</div>' + list.map(function (m) {
        var color = TOOL_COLOR[m.tool] || 'var(--muted)';
        var tint = TOOL_TINT[m.tool] || 'var(--chip)';
        return breakdownRow(color, tint, modelName(m), (Number(m.cost) || 0) / max,
          m.cost, sliceTokens(m));
      }).join('');
    }
    if (Number(s.cacheSavings) > 0.01) {
      out += '<div class="cache num">≈ ' + esc(fcost(s.cacheSavings)) +
        ' saved via cache</div>';
    }
    return out;
  }

  function byProjectBlock() {
    if (!atLeast('detailed')) return '';
    var all = summary().byProject || [];
    if (!all.length) return '';
    var max = maxCost(all);
    return '<div class="section-h">By project</div>' + all.slice(0, 5).map(function (p) {
      return breakdownRow('var(--muted)', 'var(--chip)', projectName(p),
        (Number(p.cost) || 0) / max, p.cost, sliceTokens(p));
    }).join('');
  }

  function staleBlock() {
    var s = summary();
    if (s.status !== 'stale') return '';
    return '<div class="divider"></div><div class="stale">stale · ' +
      esc(relTime(s.fetchedAt)) + (s.staleReason ? ' · ' + esc(s.staleReason) : '') +
      '</div>';
  }

  function footerBlock() {
    var ver = (state && state.version) || '';
    return '<div class="divider"></div><div class="footer">' +
      '<button class="linkbtn" data-act="wrapped">Burnt Wrapped…</button>' +
      '<button class="linkbtn plain ver" data-act="openReleases" title="View releases">' +
      (ver ? 'v' + esc(ver) : 'Burnt') + '</button></div>';
  }

  function emptyBlock() {
    return '<div class="empty"><h2>No usage yet</h2>' +
      '<p>Burnt reads the local Claude Code and Codex logs through ccusage. Start a session ' +
      'and today’s spend shows up here within a minute.</p>' +
      '<button class="btn" data-act="refresh">Refresh</button></div>';
  }

  function errorBlock() {
    var s = summary();
    return '<div class="errline">' +
      esc(s.staleReason || 'Couldn’t read usage data. Is ccusage.exe next to burnt.exe?') +
      '</div><div class="empty" style="padding-top:10px">' +
      '<button class="btn" data-act="refresh">Try again</button></div>';
  }

  function dashboardPage() {
    var st = summary().status;
    if (st === 'noData') return heroBlock() + emptyBlock() + footerBlock();
    if (st === 'error') return heroBlock() + errorBlock() + footerBlock();
    return heroBlock() + budgetBlock() + '<div class="divider"></div>' +
      statsBlock() + paceBlock() + sparkBlock() + heatBlock() +
      byToolBlock() + byModelBlock() + byProjectBlock() + staleBlock() + footerBlock();
  }

  /* ============================================================= settings */

  function selectField(label, key, options) {
    var cur = settings()[key];
    var opts = options.map(function (o) {
      return '<option value="' + esc(o[0]) + '"' + (o[0] === cur ? ' selected' : '') + '>' +
        esc(o[1]) + '</option>';
    }).join('');
    return '<div class="field"><span class="lbl">' + esc(label) + '</span>' +
      '<select data-act="select" data-key="' + esc(key) + '">' + opts + '</select></div>';
  }

  function toggleField(label, key, checked) {
    return '<label class="field sw"><span class="lbl">' + esc(label) + '</span>' +
      '<input type="checkbox" data-act="toggle" data-key="' + esc(key) + '"' +
      (checked ? ' checked' : '') + '><span class="knobtrack"></span></label>';
  }

  function updateStatusText() {
    var u = (state && state.update) || {};
    switch (u.status) {
      case 'checking': return 'Checking…';
      case 'upToDate': return 'You’re up to date';
      case 'available': return 'Update available — v' + (u.latest || '?');
      case 'updating': return 'Updating…';
      case 'error': return u.error ? 'Check failed — ' + u.error : 'Check failed';
      default: return '';
    }
  }

  function settingsPage() {
    var s = settings();
    var u = (state && state.update) || {};
    var budget = Number(s.dailyBudget) || 0;
    var h = '<div class="page-head">' +
      '<button class="iconbtn" data-act="back" title="Back" aria-label="Back">' +
      svg('back') + '</button><h1>Settings</h1></div>';

    h += selectField('Tray shows', 'menuBarMode', MENU_MODES);
    h += toggleField('Animate flame', 'animateFlame', !!s.animateFlame);
    h += '<div class="hint">Applies in icon-only mode.</div>';
    h += selectField('Dashboard style', 'dashboardStyle', STYLES);

    h += '<div class="field"><span class="lbl">Daily budget</span><span class="money">' +
      '<span class="muted">$</span><input type="text" inputmode="decimal" ' +
      'placeholder="off" data-act="budget" value="' +
      (budget > 0 ? esc(budget.toFixed(2)) : '') + '"></span></div>';
    h += '<div class="hint">0 or empty turns the budget bar off.</div>';

    h += toggleField('Launch at login', 'launchAtLogin', !!(state && state.launchAtLogin));

    h += '<div class="divider"></div><div class="grouplbl">Notifications</div>';
    h += toggleField('Budget alerts', 'notifyBudget', !!s.notifyBudget);
    h += toggleField('Daily summary', 'notifyDailySummary', !!s.notifyDailySummary);
    h += toggleField('Spend milestones', 'notifyMilestones', !!s.notifyMilestones);

    h += '<div class="divider"></div><div class="grouplbl">Updates</div>';
    h += toggleField('Automatically update Burnt', 'autoUpdate', !!s.autoUpdate);
    h += '<div class="update-row">' +
      '<button class="btn" data-act="checkUpdates"' +
      (u.status === 'checking' || u.status === 'updating' ? ' disabled' : '') +
      '>Check for Updates</button>';
    if (u.status === 'available') {
      h += '<button class="btn primary" data-act="installUpdate">Install</button>';
    }
    h += '</div>';
    var st = updateStatusText();
    if (st) {
      h += '<div class="upd-st' + (u.status === 'available' ? ' hot' : '') + '">' +
        esc(st) + '</div>';
    }

    h += '<div class="divider"></div>' +
      '<button class="linkbtn" data-act="wrapped">Burnt Wrapped…</button>' +
      '<div class="divider"></div>' +
      '<button class="linkbtn danger" data-act="quit">Quit Burnt</button>';

    var ver = (state && state.version) || '';
    if (ver) {
      h += '<div class="divider"></div><div class="footer">' +
        '<button class="linkbtn plain" data-act="openReleases">Burnt v' + esc(ver) +
        ' · release notes</button></div>';
    }
    return h;
  }

  /* ============================================================== wrapped */

  function wrappedModel() {
    var s = summary();
    var w = s.wrapped || {};
    var totals = (wrappedAllTime ? s.allTime : s.month) || {};
    var models = (w.topModels || []).slice().sort(function (a, b) {
      return (Number(b.cost) || 0) - (Number(a.cost) || 0);
    }).slice(0, 5);
    var top = Math.max((models[0] && Number(models[0].cost)) || 0, 0.0001);
    var busiest = w.busiestDay || {};
    return {
      title: wrappedAllTime ? 'All-Time' : 'This Month',
      cost: wrappedAllTime
        ? (w.allTimeCost != null ? w.allTimeCost : totals.cost)
        : (w.monthCost != null ? w.monthCost : totals.cost),
      tokens: totals.totalTokens || 0,
      bars: models.map(function (m) {
        return { name: modelName(m) || m.model, cost: Number(m.cost) || 0,
                 fraction: (Number(m.cost) || 0) / top };
      }),
      busiestDay: busiest.date ? prettyDate(busiest.date) : '—',
      busiestCost: Number(busiest.cost) || 0,
      claudeShare: Number(w.claudeShare) || 0,
      cacheSaved: Number(w.cacheSaved != null ? w.cacheSaved : s.cacheSavings) || 0
    };
  }

  function wrappedText(d) {
    var lines = ['Burnt · ' + d.title,
      fcost(d.cost) + ' · ' + ftokens(d.tokens) + ' tokens burnt', ''];
    if (d.bars.length) {
      lines.push('Top models:');
      d.bars.forEach(function (b) { lines.push('  ' + b.name + '  ' + fcost(b.cost)); });
      lines.push('');
    }
    lines.push('Busiest day:  ' + d.busiestDay + ' · ' + fcost(d.busiestCost));
    lines.push('Claude share: ' + fpercent(d.claudeShare));
    lines.push('Cache saved:  ' + fcost(d.cacheSaved));
    lines.push('');
    lines.push('How much have you burnt?  https://github.com/mafex11/Burnt');
    return lines.join('\n');
  }

  function wrappedPage() {
    var st = summary().status;
    var h = '<div class="page-head">' +
      '<button class="iconbtn" data-act="back" title="Back" aria-label="Back">' +
      svg('back') + '</button><h1>Burnt Wrapped</h1></div>';

    if (st === 'noData' || st === 'error') {
      return h + '<div class="empty"><p>No data yet — nothing to wrap up.</p></div>';
    }

    var d = wrappedModel();
    h += '<div class="seg" role="tablist">' +
      '<button role="tab" data-act="wrapMonth" aria-selected="' + (!wrappedAllTime) +
      '">This Month</button>' +
      '<button role="tab" data-act="wrapAll" aria-selected="' + (wrappedAllTime) +
      '">All-Time</button></div>';

    h += '<div class="card"><div class="ttl">Burnt · ' + esc(d.title) + '</div>' +
      '<div class="big">' + esc(fcost(d.cost)) + '</div>' +
      '<div class="toks num">' + esc(ftokens(d.tokens)) + ' tokens burnt</div>';

    if (d.bars.length) {
      h += '<div class="bars">' + d.bars.map(function (b) {
        return '<div class="mrow"><span class="mn" title="' + esc(b.name) + '">' +
          esc(b.name) + '</span><span class="mb"><i style="width:' +
          (Math.max(b.fraction, 0.02) * 100).toFixed(1) + '%"></i></span>' +
          '<span class="mc">' + esc(fcost(b.cost)) + '</span></div>';
      }).join('') + '</div>';
    }

    h += '<div class="cstats">' +
      '<div><div class="k">Busiest day</div><div class="v">' + esc(d.busiestDay) +
      ' · ' + esc(fcost(d.busiestCost)) + '</div></div>' +
      '<div><div class="k">Cache saved</div><div class="v">' + esc(fcost(d.cacheSaved)) +
      '</div></div>' +
      '<div><div class="k">Claude share</div><div class="v">' + esc(fpercent(d.claudeShare)) +
      '</div></div></div>' +
      '<div class="tag">How much have you burnt?</div></div>';

    h += '<div class="wrapped-actions">' +
      '<button class="btn" data-act="copyWrapped">Copy as text</button>';
    if (Date.now() - copiedAt < 2200) h += '<span class="copied">Copied</span>';
    h += '<span style="margin-left:auto"><button class="linkbtn plain" data-act="back">' +
      'Close</button></span></div>';
    return h;
  }

  /* =============================================================== render */

  var root = document.getElementById('root');

  function render() {
    if (!state) {
      root.innerHTML = '<div class="empty"><p>Loading…</p></div>';
      resize();
      return;
    }
    root.innerHTML = page === 'settings' ? settingsPage()
      : page === 'wrapped' ? wrappedPage()
      : dashboardPage();
    resize();
  }

  /* Report the content height so Go can size the frameless window.
     NOTE: `document.documentElement.scrollHeight` can't be used directly — it is
     floored at the viewport height, so a short dashboard inside a tall window
     would keep reporting the old (too large) height and never shrink. #root is
     the only child of <body> and body has no padding/margin, so its box height
     *is* the content height, and it equals documentElement.scrollHeight whenever
     the content actually overflows. */
  function resize() {
    requestAnimationFrame(function () {
      var content = Math.ceil(root.getBoundingClientRect().height);
      var h = Math.max(1, Math.min(content, document.documentElement.scrollHeight));
      document.documentElement.classList.toggle('scrolls', h > MAX_HEIGHT);
      call('burnt_resize', Math.min(h, MAX_HEIGHT));
    });
  }

  /* ============================================================== actions */

  function refresh() {
    if (state) { state.loading = true; render(); }
    call('burnt_refresh').then(function () {
      // Go normally pushes the fresh state via burntOnState() while the promise
      // is in flight. Only pull it ourselves if that didn't happen, so a normal
      // refresh costs one repaint instead of three.
      if (state && !state.loading) return;
      return call('burnt_getState').then(function (json) {
        if (json) window.burntOnState(json);
        else if (state) { state.loading = false; render(); }
      });
    }).catch(function () {
      if (state) { state.loading = false; render(); }
    });
  }

  var ACTIONS = {
    settings: function () { page = 'settings'; render(); },
    back: function () { page = 'dashboard'; render(); },
    wrapped: function () { page = 'wrapped'; render(); },
    refresh: refresh,
    quit: function () { call('burnt_quit'); },
    openReleases: function () {
      call('burnt_openUrl', 'https://github.com/mafex11/Burnt/releases');
    },
    checkUpdates: function () {
      if (state) { state.update = { status: 'checking', latest: null, error: '' }; render(); }
      call('burnt_checkUpdates').then(function () { return call('burnt_getState'); })
        .then(function (json) { if (json) window.burntOnState(json); });
    },
    installUpdate: function () {
      if (state) {
        state.update = state.update || {};
        state.update.status = 'updating';
        render();
      }
      call('burnt_installUpdate');
    },
    wrapMonth: function () { wrappedAllTime = false; render(); },
    wrapAll: function () { wrappedAllTime = true; render(); },
    copyWrapped: function () {
      call('burnt_copyText', wrappedText(wrappedModel()));
      copiedAt = Date.now();
      render();
      setTimeout(render, 2300);
    }
  };

  root.addEventListener('click', function (ev) {
    var el = ev.target.closest('[data-act]');
    if (!el) return;
    var act = el.getAttribute('data-act');
    if (ACTIONS[act]) { ev.preventDefault(); ACTIONS[act](); }
  });

  root.addEventListener('change', function (ev) {
    var el = ev.target.closest('[data-act]');
    if (!el) return;
    var act = el.getAttribute('data-act');
    var key = el.getAttribute('data-key');
    if (act === 'select') { patch(key, el.value); render(); return; }
    if (act === 'toggle') {
      if (key === 'launchAtLogin') {
        if (state) state.launchAtLogin = el.checked;
        call('burnt_setLaunchAtLogin', el.checked);
        render();
        return;
      }
      patch(key, el.checked);
      render();
    }
  });

  /* Budget commits on every keystroke (like SwiftUI's onChange) but must NOT
     re-render — that would blow away the caret mid-typing. */
  root.addEventListener('input', function (ev) {
    var el = ev.target.closest('[data-act="budget"]');
    if (!el) return;
    var cleaned = el.value.replace(/[$,\s]/g, '');
    var v = parseFloat(cleaned);
    patch('dailyBudget', isNaN(v) || v < 0 ? 0 : v);
  });

  /* --------------------------------------------------------- hover captions */

  function hoverCaption(group, idx) {
    var capEl = document.getElementById(group === 'spark' ? 'spark-cap' : 'heat-cap');
    if (!capEl) return;
    var pts = group === 'spark' ? sparkPoints() : heatPoints();
    if (idx == null || isNaN(idx) || !pts[idx]) {
      capEl.textContent = group === 'spark' ? 'last 14 days' : '';
      capEl.classList.remove('live');
    } else {
      var p = pts[idx];
      capEl.textContent = prettyDate(p.date) + ' · ' + fcost(p.cost);
      capEl.classList.add('live');
    }
  }

  root.addEventListener('mouseover', function (ev) {
    var el = ev.target.closest('[data-hov]');
    if (!el) return;
    el.classList.add('on');
    hoverCaption(el.getAttribute('data-hov'), parseInt(el.getAttribute('data-i'), 10));
  });

  root.addEventListener('mouseout', function (ev) {
    var el = ev.target.closest('[data-hov]');
    if (!el) return;
    el.classList.remove('on');
    var group = el.getAttribute('data-hov');
    var to = ev.relatedTarget && ev.relatedTarget.closest
      ? ev.relatedTarget.closest('[data-hov]') : null;
    // Reset unless the pointer moved to another cell in the *same* chart —
    // jumping from the heatmap to the sparkline must clear the heatmap caption.
    if (!to || to.getAttribute('data-hov') !== group) hoverCaption(group, null);
  });

  /* ================================================================= boot */

  call('burnt_getState').then(function (json) {
    if (json) window.burntOnState(json);
    else render();
  }).catch(render);

  render();
  window.addEventListener('resize', resize);
})();
