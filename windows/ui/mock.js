/* Burnt for Windows — browser-only fixture bridge.
 *
 * When index.html is opened in an ordinary browser there is no Go host, so the
 * burnt_* globals don't exist. This file fills them in with realistic data so
 * the dashboard can be developed and screenshotted without Windows.
 *
 * Numbers are derived from Tests/UsageEngineTests/Fixtures/daily-normal.json
 * (mixed Claude + Codex models, all-time totals taken verbatim) plus 84 days of
 * deterministic pseudo-random daily costs, so screenshots are stable.
 *
 * Preview params:
 *   ?style=minimal|standard|detailed     dashboard density        (default standard)
 *   ?status=ok|stale|noData|error        summary status           (default ok)
 *   ?theme=light|dark                    force a theme            (default: system)
 *   ?page=settings|wrapped               initial page             (default dashboard)
 *   ?update=idle|checking|upToDate|available|updating|error
 *   ?budget=<dollars>                    daily budget             (default: ~60% of today)
 *
 * It is a no-op when the real bridge is present.
 */
(function () {
  'use strict';
  if (typeof window.burnt_getState === 'function') return;   // real host wins

  var P = new URLSearchParams(location.search);
  var STYLE = P.get('style') || 'standard';
  var STATUS = P.get('status') || 'ok';

  /* --------------------------------------------------------------- helpers */

  /* Deterministic LCG so the fixture (and therefore screenshots) never drift. */
  function lcg(seed) {
    var s = seed >>> 0;
    return function () {
      s = (Math.imul(s, 1103515245) + 12345) & 0x7fffffff;
      return s / 0x7fffffff;
    };
  }

  function iso(d) {
    return d.getFullYear() + '-' +
      String(d.getMonth() + 1).padStart(2, '0') + '-' +
      String(d.getDate()).padStart(2, '0');
  }

  /* Token split roughly matching the fixture's Claude row: cache-read heavy. */
  var TOKENS_PER_DOLLAR = 1450000;
  function totalsFor(cost) {
    var total = Math.round(cost * TOKENS_PER_DOLLAR);
    return {
      cost: cost,
      inputTokens: Math.round(total * 0.040),
      outputTokens: Math.round(total * 0.010),
      cacheCreationTokens: Math.round(total * 0.050),
      cacheReadTokens: Math.round(total * 0.900),
      totalTokens: total
    };
  }

  function sumTotals(costs) {
    return totalsFor(costs.reduce(function (a, b) { return a + b; }, 0));
  }

  /* ------------------------------------------------------- 84 days of cost */

  var rand = lcg(20260906);
  var days = [];
  var start = new Date();
  start.setHours(12, 0, 0, 0);
  start.setDate(start.getDate() - 83);

  for (var i = 0; i < 84; i++) {
    var d = new Date(start);
    d.setDate(start.getDate() + i);
    var r = rand(), r2 = rand();
    var weekend = d.getDay() === 0 || d.getDay() === 6;
    var cost;
    if (r < 0.09) {
      cost = 0;                                   // idle day
    } else {
      // Weekday spend lands around $15-$110, in the same ballpark as the
      // $21.67 and $258.56 rows in daily-normal.json.
      cost = (weekend ? 13 : 58) * (0.25 + r2 * 1.75);
      if (r > 0.945) cost *= 3.2;                 // a monster day; exercises the heat ramp
    }
    if (i === 83) cost = 18.42;                   // today, only partway through
    days.push({ date: iso(d), cost: Math.round(cost * 100) / 100 });
  }

  var costs = days.map(function (p) { return p.cost; });
  var todayCost = costs[83];
  var weekCosts = costs.slice(77);          // rolling 7 days ending today
  var lastWeekCosts = costs.slice(70, 77);  // the 7 before that
  var thisMonth = days.filter(function (p) {
    return p.date.slice(0, 7) === days[83].date.slice(0, 7);
  }).map(function (p) { return p.cost; });

  var today = totalsFor(todayCost);
  var week = sumTotals(weekCosts);
  var lastWeek = sumTotals(lastWeekCosts);
  var month = sumTotals(thisMonth);

  /* All-time comes straight from daily-normal.json's `totals`. */
  var allTime = {
    cost: 7468.346583849998,
    inputTokens: 683093270,
    outputTokens: 31975228,
    cacheCreationTokens: 306159669,
    cacheReadTokens: 11078379056,
    totalTokens: 12099607223
  };

  /* ------------------------------------------------- week-window breakdowns */

  var MODEL_MIX = [
    ['claude-opus-4-7', 'claude', 0.55],
    ['gpt-5.2-codex', 'codex', 0.18],
    ['claude-sonnet-4-6', 'claude', 0.17],
    ['claude-haiku-4-5-20251001', 'claude', 0.06],
    ['gpt-5.2-codex-mini', 'codex', 0.04]
  ];

  var byModel = MODEL_MIX.map(function (m) {
    var c = Math.round(week.cost * m[2] * 100) / 100;
    return { model: m[0], tool: m[1], cost: c, tokens: Math.round(c * TOKENS_PER_DOLLAR) };
  });

  function toolSlice(tool) {
    var c = 0, t = 0;
    byModel.forEach(function (m) { if (m.tool === tool) { c += m.cost; t += m.tokens; } });
    return { tool: tool, cost: Math.round(c * 100) / 100, tokens: t };
  }
  var byTool = [toolSlice('claude'), toolSlice('codex')];

  var byProject = [
    ['burnt', 0.34], ['yuki', 0.26], ['maxmi', 0.18], ['sendrn', 0.14], ['Unknown', 0.08]
  ].map(function (p) {
    var c = Math.round(week.cost * p[1] * 100) / 100;
    return { project: p[0], cost: c, tokens: Math.round(c * TOKENS_PER_DOLLAR) };
  });

  var cacheSavings = Math.round(week.cost * 1.9 * 100) / 100;

  var busiest = days.reduce(function (a, b) { return b.cost > a.cost ? b : a; }, days[0]);
  var claudeShare = byTool[0].cost / Math.max(byTool[0].cost + byTool[1].cost, 0.0001);

  /* Pace uses a fixed fraction-of-day so previews stay reproducible. */
  var projectedToday = todayCost > 0 ? Math.round((todayCost / 0.55) * 100) / 100 : null;

  var okSummary = {
    status: 'ok',
    staleReason: '',
    fetchedAt: new Date().toISOString(),
    today: today,
    week: week,
    month: month,
    allTime: allTime,
    lastWeek: lastWeek,
    avgPerDay: Math.round((week.cost / 7) * 100) / 100,
    weekTrend: lastWeek.cost > 0
      ? Math.round(((week.cost - lastWeek.cost) / lastWeek.cost) * 1000) / 1000 : null,
    projectedToday: projectedToday,
    sparkline: days.slice(70),
    heatmap: days,
    byTool: byTool,
    byModel: byModel,
    byProject: byProject,
    cacheSavings: cacheSavings,
    wrapped: {
      monthCost: month.cost,
      allTimeCost: allTime.cost,
      topModels: byModel.map(function (m) { return { model: m.model, cost: m.cost }; }),
      busiestDay: { date: busiest.date, cost: busiest.cost },
      claudeShare: Math.round(claudeShare * 1000) / 1000,
      cacheSaved: cacheSavings
    }
  };

  var zero = totalsFor(0);
  var emptySummary = {
    status: 'noData', staleReason: '', fetchedAt: new Date().toISOString(),
    today: zero, week: zero, month: zero, allTime: zero, lastWeek: zero,
    avgPerDay: 0, weekTrend: null, projectedToday: null,
    sparkline: days.map(function (p) { return { date: p.date, cost: 0 }; }).slice(70),
    heatmap: days.map(function (p) { return { date: p.date, cost: 0 }; }),
    byTool: [], byModel: [], byProject: [], cacheSavings: 0,
    wrapped: {
      monthCost: 0, allTimeCost: 0, topModels: [],
      busiestDay: { date: '', cost: 0 }, claudeShare: 0, cacheSaved: 0
    }
  };

  function summaryFor(status) {
    if (status === 'noData') return emptySummary;
    var s = JSON.parse(JSON.stringify(okSummary));
    if (status === 'stale') {
      s.status = 'stale';
      s.staleReason = 'ccusage timed out';
      s.fetchedAt = new Date(Date.now() - 41 * 60 * 1000).toISOString();
    } else if (status === 'error') {
      s.status = 'error';
      s.staleReason = 'ccusage.exe exited with code 1: EPERM reading %USERPROFILE%\\.claude';
    }
    return s;
  }

  /* ---------------------------------------------------------------- state */

  var budgetParam = parseFloat(P.get('budget'));
  var defaultBudget = Math.max(5, Math.round(todayCost / 0.62 / 5) * 5);

  var st = {
    summary: summaryFor(STATUS),
    settings: {
      menuBarMode: 'todayCost',
      dailyBudget: isNaN(budgetParam) ? defaultBudget : budgetParam,
      dashboardStyle: STYLE,
      notifyBudget: true,
      notifyDailySummary: false,
      notifyMilestones: true,
      animateFlame: true,
      autoUpdate: true,
      lastUpdateCheck: new Date(Date.now() - 3 * 3600 * 1000).toISOString()
    },
    version: '1.3.0',
    update: { status: P.get('update') || 'idle', latest: null, error: '' },
    loading: false,
    launchAtLogin: true
  };
  if (st.update.status === 'available') st.update.latest = '1.4.0';
  if (st.update.status === 'error') st.update.error = 'network unreachable';

  function push() {
    if (typeof window.burntOnState === 'function') {
      window.burntOnState(JSON.stringify(st));
    }
  }

  function log() { console.log.apply(console, ['[mock]'].concat([].slice.call(arguments))); }

  /* --------------------------------------------------------------- bridge */

  window.burnt_getState = function () { return Promise.resolve(JSON.stringify(st)); };

  window.burnt_refresh = function () {
    st.loading = true;
    push();
    return new Promise(function (resolve) {
      setTimeout(function () {
        // Nudge today's spend so a refresh visibly does something.
        var bump = Math.round((0.4 + Math.random() * 2.4) * 100) / 100;
        var c = Math.round((st.summary.today.cost + bump) * 100) / 100;
        st.summary.today = totalsFor(c);
        st.summary.heatmap[83] = { date: st.summary.heatmap[83].date, cost: c };
        st.summary.sparkline[13] = { date: st.summary.sparkline[13].date, cost: c };
        st.summary.fetchedAt = new Date().toISOString();
        st.loading = false;
        push();
        resolve('');
      }, 600);
    });
  };

  window.burnt_setSettings = function (json) {
    try { st.settings = JSON.parse(json); } catch (e) { log('bad settings json', e); }
    log('setSettings', st.settings);
    return Promise.resolve('');
  };

  window.burnt_setLaunchAtLogin = function (on) {
    st.launchAtLogin = (on === true || on === 'true');
    log('setLaunchAtLogin', st.launchAtLogin);
    return Promise.resolve('');
  };

  window.burnt_getLaunchAtLogin = function () {
    return Promise.resolve(String(st.launchAtLogin));
  };

  window.burnt_checkUpdates = function () {
    st.update = { status: 'checking', latest: null, error: '' };
    push();
    return new Promise(function (resolve) {
      setTimeout(function () {
        st.update = P.get('update') === 'upToDate'
          ? { status: 'upToDate', latest: null, error: '' }
          : { status: 'available', latest: '1.4.0', error: '' };
        push();
        resolve('');
      }, 700);
    });
  };

  window.burnt_installUpdate = function () {
    st.update = { status: 'updating', latest: st.update.latest, error: '' };
    push();
    return Promise.resolve('');
  };

  window.burnt_openUrl = function (url) { log('openUrl', url); return Promise.resolve(''); };

  window.burnt_copyText = function (text) {
    log('copyText\n' + text);
    if (navigator.clipboard) navigator.clipboard.writeText(text).catch(function () {});
    return Promise.resolve('');
  };

  window.burnt_quit = function () { log('quit'); return Promise.resolve(''); };

  window.burnt_resize = function (h) {
    log('resize', h);
    document.title = 'Burnt — ' + h + 'px';
    return Promise.resolve('');
  };
})();
