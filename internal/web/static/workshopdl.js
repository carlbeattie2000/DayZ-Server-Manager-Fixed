// Copyright (c) 2026 Aristarh Ucolov.
//
// Mods → Direct download: Steam Workshop mods straight onto the server with
// Valve's SteamCMD — no DayZ client, no download folder. Three steps: get
// SteamCMD, sign in once with a Steam account that owns DayZ (the password
// goes to SteamCMD only; it keeps its own login token), paste Workshop links.
// The backend is internal/web/workshop_dl.go.

const WSDL_MAX_MODS = 200;
const WSDL_POLL_MS = 1500;
const WSDL_NAVIGATE_DELAY_MS = 600;
const WSDL_MOD_ENTRY_PATTERN = /[^\s,;]+/g;
const WSDL_ITEM_JOB_KINDS = new Set(['download', 'sync']);
const WSDL_AUTH_JOB_KINDS = new Set(['login', 'check']);

function wsdlEl(tag, options = {}, ...children) {
  const { className, text, i18n, attrs, style, ...props } = options;
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (i18n) {
    node.dataset.i18n = i18n;
    node.textContent = t(i18n);
  }
  if (text !== undefined) node.textContent = text;
  Object.assign(node, props);
  if (style) Object.assign(node.style, style);
  for (const [name, value] of Object.entries(attrs ?? {})) node.setAttribute(name, value);
  node.append(...children.flat().filter(Boolean));
  return node;
}

function wsdlSetVisible(node, visible, display = '') {
  node.style.display = visible ? display : 'none';
}

function wsdlCountModEntries(text) {
  return new Set(text.match(WSDL_MOD_ENTRY_PATTERN) ?? []).size;
}

function wsdlFormatBytes(value) {
  return value ? bytes(value) : '—';
}

function wsdlCreateRegion(container) {
  const anchor = document.createComment('');
  container.append(anchor);
  let current = [];

  return {
    set(nodes) {
      current.forEach((node) => node.remove());
      current = nodes.filter(Boolean);
      anchor.before(...current);
    },
  };
}

function wsdlCreateStep(number, titleKey) {
  const marker = wsdlEl('span', { className: 'wsdl-n', text: String(number) });
  const content = wsdlEl('div', { className: 'wsdl-step-main' },
    wsdlEl('div', { className: 'wsdl-step-title', i18n: titleKey }));
  const root = wsdlEl('div', { className: 'wsdl-step todo' }, marker, content);

  return {
    root,
    content,
    setDone(done) {
      root.className = `wsdl-step ${done ? 'ok' : 'todo'}`;
      marker.textContent = done ? '✓' : String(number);
    },
  };
}

function workshopDownloadCard() {
  const card = wsdlEl('div', { className: 'card wsdl', id: 'workshop-download' },
    cardTitle('i-mods', 'wsdl.title', 'wsdl'),
    wsdlEl('p', { className: 'hint', i18n: 'wsdl.hint' }));

  const state = {
    status: null,
    found: [],
    picked: new Set(),
    codeNeeded: '',
    lastUser: '',
    lastPass: '',
    links: '',
    serverSide: false,
    watching: false,
    pollTimer: 0,
  };

  const isBusy = () => state.status?.job?.state === 'running';
  const isSignedIn = () => Boolean(state.status?.user) && state.status?.loggedIn === true;

  const loadError = wsdlEl('div', { className: 'warning-bar err-bar' });
  wsdlSetVisible(loadError, false);

  const toolStep = wsdlCreateStep(1, 'wsdl.step.tool');
  const toolInstalled = wsdlEl('div', { className: 'hint' });
  const toolNeed = wsdlEl('p', { className: 'hint', i18n: 'wsdl.tool.need' });
  const toolInstallButton = wsdlEl('button', {
    className: 'primary',
    i18n: 'wsdl.tool.install',
    onclick: (event) => withBusy(event.currentTarget, () => watch(api.post('/api/workshop/dl/setup', {}))),
  });
  const toolProgress = wsdlEl('div', { className: 'wsdl-progress', i18n: 'wsdl.tool.installing' });
  const toolError = wsdlEl('div', { className: 'warning-bar err-bar' });
  toolStep.content.append(toolInstalled, toolNeed, toolInstallButton, toolProgress, toolError);

  const loginStep = wsdlCreateStep(2, 'wsdl.step.login');
  const loginAfterTool = wsdlEl('p', { className: 'hint', i18n: 'wsdl.login.afterTool' });

  const signedLabel = wsdlEl('span');
  const checkButton = wsdlEl('button', {
    className: 'secondary small',
    i18n: 'wsdl.login.check',
    onclick: (event) => withBusy(event.currentTarget, () => watch(api.post('/api/workshop/dl/check', {}))),
  });
  const forgetButton = wsdlEl('button', {
    className: 'secondary small',
    i18n: 'wsdl.login.forget',
    onclick: (event) => withBusy(event.currentTarget, forgetLogin),
  });
  const signedPanel = wsdlEl('div', { className: 'wsdl-signed' }, signedLabel, checkButton, forgetButton);

  const userInput = wsdlEl('input', {
    type: 'text',
    autocomplete: 'username',
    placeholder: t('wsdl.login.user'),
    attrs: { 'aria-label': t('wsdl.login.user') },
  });
  const passInput = wsdlEl('input', {
    type: 'password',
    autocomplete: 'current-password',
    placeholder: t('wsdl.login.pass'),
    attrs: { 'aria-label': t('wsdl.login.pass') },
  });
  const codeInput = wsdlEl('input', {
    type: 'text',
    inputMode: 'text',
    maxLength: 10,
    autocomplete: 'one-time-code',
    className: 'mono wsdl-code',
    attrs: { 'aria-label': t('wsdl.code.label') },
  });
  const signInButton = wsdlEl('button', { className: 'primary', i18n: 'wsdl.login.signin', onclick: submitLogin });
  const sendCodeButton = wsdlEl('button', { className: 'primary', i18n: 'wsdl.code.send', onclick: submitLogin });
  const cancelCodeButton = wsdlEl('button', {
    className: 'linkish',
    i18n: 'action.cancel',
    onclick: () => {
      state.codeNeeded = '';
      state.lastPass = '';
      codeInput.value = '';
      render();
    },
  });
  const insecureWarning = wsdlEl('p', { className: 'warning-bar', i18n: 'wsdl.login.insecure' });
  const codeHint = wsdlEl('p', { className: 'info-bar' });
  const codeRow = wsdlEl('div', { className: 'row wsdl-row' }, codeInput, sendCodeButton, cancelCodeButton);
  const credentialsRow = wsdlEl('div', { className: 'row wsdl-row' }, userInput, passInput, signInButton);
  const loginWhy = wsdlEl('small', { className: 'hint', i18n: 'wsdl.login.why' });
  loginStep.content.append(
    loginAfterTool,
    signedPanel,
    insecureWarning,
    codeHint,
    codeRow,
    credentialsRow,
    loginWhy,
  );
  const loginFeedback = wsdlCreateRegion(loginStep.content);

  const modsStep = wsdlCreateStep(3, 'wsdl.step.mods');

  const linksInput = wsdlEl('textarea', {
    rows: 3,
    className: 'mono',
    placeholder: t('wsdl.links.ph'),
    attrs: { 'aria-label': t('wsdl.links.ph') },
    oninput: () => {
      state.links = linksInput.value;
      updateModCountWarning();
    },
  });
  const modCountWarning = wsdlEl('div', {
    className: 'warning-bar err-bar',
    text: `Warning: More than ${WSDL_MAX_MODS} unique mods detected. Only the first ${WSDL_MAX_MODS} will be processed by the server.`,
  });
  const lookupButton = wsdlEl('button', {
    i18n: 'wsdl.links.lookup',
    onclick: (event) => withBusy(event.currentTarget, lookupLinks),
  });
  const updatesButton = wsdlEl('button', {
    className: 'secondary',
    i18n: 'wsdl.updates',
    onclick: (event) => withBusy(event.currentTarget, checkUpdates),
  });
  modsStep.content.append(
    linksInput,
    modCountWarning,
    wsdlEl('div', { className: 'row wsdl-row' }, lookupButton, updatesButton),
  );
  const resultsRegion = wsdlCreateRegion(modsStep.content);
  const jobRegion = wsdlCreateRegion(modsStep.content);

  const stepRoots = [toolStep.root, loginStep.root, modsStep.root];
  stepRoots.forEach((root) => wsdlSetVisible(root, false));
  card.append(wsdlEl('div', { className: 'wsdl-body' }, loadError, ...stepRoots));

  updateModCountWarning();

  function updateModCountWarning() {
    wsdlSetVisible(modCountWarning, wsdlCountModEntries(linksInput.value) > WSDL_MAX_MODS, 'block');
  }

  async function load() {
    try {
      state.status = await api.get('/api/workshop/dl/status');
    } catch (error) {
      loadError.textContent = error.message;
      wsdlSetVisible(loadError, true);
      return;
    }
    wsdlSetVisible(loadError, false);

    const running = isBusy();
    clearTimeout(state.pollTimer);
    if (running && card.isConnected) state.pollTimer = setTimeout(load, WSDL_POLL_MS);

    if (!running && state.status.job) handleFinishedJob(state.status.job);
    render();
  }

  function handleFinishedJob(job) {
    if (!state.watching) return;

    if (WSDL_ITEM_JOB_KINDS.has(job.kind)) {
      state.watching = false;
      const items = job.items ?? [];
      const doneCount = items.filter((item) => item.state === 'done').length;
      const isSync = job.kind === 'sync';

      if (isSync && !items.length && !job.error) {
        toast(t('wsdl.updates.none'), 'ok');
      } else if (doneCount) {
        toast(tn(isSync ? 'wsdl.updatedN' : 'wsdl.installedN', { n: doneCount }), 'ok');
        setTimeout(() => navigate('mods'), WSDL_NAVIGATE_DELAY_MS);
      }
      return;
    }

    if (WSDL_AUTH_JOB_KINDS.has(job.kind)) {
      state.watching = false;
      const result = job.login ?? {};
      if (result.ok) {
        state.lastPass = '';
        state.codeNeeded = '';
        passInput.value = '';
        codeInput.value = '';
        toast(t('wsdl.login.ok'), 'ok');
      } else if (result.need) {
        state.codeNeeded = result.need;
      }
    }
  }

  async function watch(request) {
    try {
      await request;
      state.watching = true;
      await load();
    } catch (error) {
      handleErr(error);
    }
  }

  async function forgetLogin() {
    const confirmed = await confirmModal(t('wsdl.login.forgetConfirm'), { okText: t('wsdl.login.forget') });
    if (!confirmed) return;
    try {
      await api.post('/api/workshop/dl/forget', {});
      await load();
    } catch (error) {
      handleErr(error);
    }
  }

  async function submitLogin(event) {
    return withBusy(event.currentTarget, async () => {
      const username = userInput.value.trim();
      const password = state.codeNeeded ? state.lastPass : passInput.value;
      if (!username || !password) {
        toast(t('wsdl.login.needBoth'), 'error');
        return;
      }
      state.lastUser = username;
      state.lastPass = password;
      await watch(api.post('/api/workshop/dl/login', {
        user: username,
        password,
        code: state.codeNeeded ? codeInput.value.trim() : '',
      }));
    });
  }

  async function lookupLinks() {
    try {
      const response = await api.post('/api/workshop/dl/lookup', { text: linksInput.value });
      state.found = response.items ?? [];
      state.picked = new Set(
        state.found.filter((item) => item.found && item.dayz && !item.installed).map((item) => item.id),
      );
      render();
    } catch (error) {
      handleErr(error);
    }
  }

  async function checkUpdates() {
    try {
      const response = await api.post('/api/workshop/dl/updates', {});
      state.found = (response.mods ?? []).map((mod) => ({
        id: mod.id,
        title: mod.title,
        size: mod.size,
        updated: mod.updated,
        found: true,
        dayz: true,
        installed: mod.mod,
        newer: mod.newer,
      }));
      state.picked = new Set(state.found.filter((item) => item.newer).map((item) => item.id));
      if (!state.found.some((item) => item.newer)) toast(t('wsdl.updates.none'), 'ok');
      render();
    } catch (error) {
      handleErr(error);
    }
  }

  function startDownload(items) {
    return watch(api.post('/api/workshop/dl/download', {
      items: items.map(({ id, title, size }) => ({ id, title, size })),
      serverSide: state.serverSide,
    }));
  }

  async function clearJobItems(id) {
    try {
      await api.post('/api/workshop/dl/clear', id ? { id } : {});
      await load();
    } catch (error) {
      handleErr(error);
    }
  }

  function renderToolStep() {
    const { installed, dir, job } = state.status;
    const busy = isBusy();
    const isSetupJob = job?.kind === 'setup';

    toolStep.setDone(installed);
    toolInstalled.textContent = t('wsdl.tool.ok').replace('{dir}', dir);
    wsdlSetVisible(toolInstalled, installed);
    wsdlSetVisible(toolNeed, !installed);
    wsdlSetVisible(toolInstallButton, !installed);
    toolInstallButton.disabled = busy;
    wsdlSetVisible(toolProgress, isSetupJob && busy);

    const failed = isSetupJob && job.state === 'error';
    toolError.textContent = failed ? t('wsdl.tool.failed').replace('{err}', job.error || '') : '';
    wsdlSetVisible(toolError, failed);
  }

  function renderLoginStep() {
    const { installed, user, secure, job } = state.status;
    const busy = isBusy();
    const signedIn = isSignedIn();
    const showSigned = installed && signedIn && !state.codeNeeded;
    const showForm = installed && !showSigned;
    const needsCode = Boolean(state.codeNeeded);

    loginStep.setDone(signedIn);
    wsdlSetVisible(loginAfterTool, !installed);

    wsdlSetVisible(signedPanel, showSigned);
    signedLabel.textContent = t('wsdl.login.as').replace('{user}', user ?? '');
    checkButton.disabled = busy;
    forgetButton.disabled = busy;

    wsdlSetVisible(insecureWarning, showForm && !secure);
    wsdlSetVisible(codeHint, showForm && needsCode);
    codeHint.dataset.i18n = state.codeNeeded === '2fa' ? 'wsdl.code.2faHint' : 'wsdl.code.mailHint';
    codeHint.textContent = t(codeHint.dataset.i18n);
    wsdlSetVisible(codeRow, showForm && needsCode);
    wsdlSetVisible(credentialsRow, showForm && !needsCode);
    wsdlSetVisible(loginWhy, showForm);
    codeInput.placeholder = t(state.codeNeeded === '2fa' ? 'wsdl.code.2fa' : 'wsdl.code.mail');
    if (!userInput.value) userInput.value = state.lastUser || user || '';
    signInButton.disabled = busy || !secure;
    sendCodeButton.disabled = busy || !secure;

    const isAuthJob = job && WSDL_AUTH_JOB_KINDS.has(job.kind);
    const nodes = [];
    if (isAuthJob && busy) {
      nodes.push(wsdlEl('div', { className: 'wsdl-progress', i18n: 'wsdl.login.waiting' }));
    } else if (isAuthJob && job.login && !job.login.ok && !job.login.need) {
      const reason = t(`wsdl.login.err.${job.login.reason || 'unknown'}`);
      const detail = job.login.detail ? ` (${job.login.detail})` : '';
      nodes.push(wsdlEl('div', { className: 'warning-bar err-bar', text: reason + detail }));
    }
    loginFeedback.set(nodes);
  }

  function createFoundItem(item) {
    const selectable = item.found && item.dayz;
    const checkbox = wsdlEl('input', {
      type: 'checkbox',
      disabled: !selectable,
      checked: state.picked.has(item.id),
      onchange: () => {
        if (checkbox.checked) state.picked.add(item.id);
        else state.picked.delete(item.id);
        render();
      },
    });

    const meta = [item.id];
    if (item.size) meta.push(wsdlFormatBytes(item.size));
    if (item.updated) meta.push(fmtDateTime(item.updated));

    const installedBadge = item.installed
      ? wsdlEl('span', {
        className: `badge ${item.newer ? 'warn' : 'mute'}`,
        text: (item.newer ? `${t('wsdl.item.newer')} · ` : '') + item.installed,
      })
      : null;
    const notDayzBadge = item.found && !item.dayz
      ? wsdlEl('span', { className: 'badge err', i18n: 'wsdl.item.notDayz' })
      : null;

    return wsdlEl('label', { className: `wsdl-item${selectable ? '' : ' bad'}` },
      checkbox,
      wsdlEl('span', { className: 'wsdl-item-main' },
        wsdlEl('b', { text: item.found ? item.title : t('wsdl.item.notFound') }),
        wsdlEl('span', { className: 'hint mono', text: meta.join(' · ') })),
      installedBadge,
      notDayzBadge);
  }

  function renderResults() {
    if (!state.found.length) {
      resultsRegion.set([]);
      return;
    }

    const busy = isBusy();
    const selected = state.found.filter((item) => state.picked.has(item.id));
    const totalSize = selected.reduce((sum, item) => sum + (item.size || 0), 0);

    const serverSideToggle = wsdlEl('input', {
      type: 'checkbox',
      className: 'switch',
      checked: state.serverSide,
      onchange: () => { state.serverSide = serverSideToggle.checked; },
    });
    const clearButton = wsdlEl('button', {
      className: 'linkish',
      i18n: 'wsdl.list.clear',
      disabled: busy,
      onclick: () => {
        state.found = [];
        state.picked.clear();
        render();
      },
    });
    const downloadButton = wsdlEl('button', {
      className: 'primary',
      disabled: busy || !selected.length || !isSignedIn() || state.status.running,
      text: tn('wsdl.download', { n: selected.length }) + (totalSize ? ` · ${bytes(totalSize)}` : ''),
      onclick: (event) => withBusy(event.currentTarget, () => startDownload(selected)),
    });

    resultsRegion.set([
      wsdlEl('div', { className: 'wsdl-list' }, ...state.found.map(createFoundItem)),
      wsdlEl('div', { className: 'row wsdl-row' },
        wsdlEl('label', { className: 'inline-switch' }, serverSideToggle, wsdlEl('span', { i18n: 'wsdl.serverSide' })),
        wsdlEl('span', { className: 'grow' }),
        clearButton,
        downloadButton),
      isSignedIn() ? null : wsdlEl('small', { className: 'hint', i18n: 'wsdl.needLogin' }),
      state.status.running ? wsdlEl('small', { className: 'hint warn-text', i18n: 'wsdl.stopFirst' }) : null,
    ]);
  }

  function createJobRow(item) {
    const busy = isBusy();
    const canRetry = !busy && isSignedIn() && !state.status.running;
    const percent = item.size
      ? Math.min(100, Math.round(((item.bytes || 0) * 100) / item.size))
      : (item.state === 'done' ? 100 : 0);
    const stateLabel = item.gone
      ? t('wsdl.state.gone').replace('{mod}', item.mod)
      : t(`wsdl.state.${item.state}`) + (item.mod ? ` → ${item.mod}` : '') + (item.error ? ` · ${item.error}` : '');
    const displayName = item.title || item.id;
    const sizeLabel = wsdlFormatBytes(item.bytes) + (item.size ? ` / ${wsdlFormatBytes(item.size)}` : '');

    const progress = item.gone
      ? wsdlEl('button', {
        className: 'secondary small',
        i18n: 'wsdl.again',
        disabled: !canRetry,
        onclick: () => withBusyAll(() => startDownload([item])),
      })
      : wsdlEl('div', { className: 'progress' },
        wsdlEl('div', { className: 'progress-bar', style: { width: `${item.state === 'done' ? 100 : percent}%` } }));

    const removeButton = busy
      ? null
      : wsdlEl('button', {
        className: 'icon-x',
        text: '×',
        title: t('wsdl.list.remove'),
        attrs: { 'aria-label': `${t('wsdl.list.remove')}: ${displayName}` },
        onclick: () => withBusyAll(() => clearJobItems(item.id)),
      });

    return wsdlEl('div', { className: `wsdl-job-row ${item.state}${item.gone ? ' gone' : ''}` },
      wsdlEl('span', { className: 'wsdl-job-name', text: displayName }),
      wsdlEl('span', { className: 'wsdl-job-state', text: stateLabel }),
      progress,
      wsdlEl('span', { className: 'hint mono', text: sizeLabel }),
      removeButton);
  }

  function renderJob() {
    const job = state.status.job;
    if (!job || !WSDL_ITEM_JOB_KINDS.has(job.kind)) {
      jobRegion.set([]);
      return;
    }

    const busy = isBusy();
    const items = job.items ?? [];
    const nodes = [];

    if (job.kind === 'sync') {
      nodes.push(wsdlEl('div', {
        className: `wsdl-progress${busy ? '' : ' done'}`,
        i18n: busy ? 'wsdl.sync.running' : 'wsdl.sync.done',
      }));
    }

    const rows = wsdlEl('div', { className: 'wsdl-job' }, ...items.map(createJobRow));
    if (!busy && items.length) {
      rows.append(wsdlEl('div', { className: 'wsdl-job-foot' },
        wsdlEl('span', { className: 'grow' }),
        wsdlEl('button', {
          className: 'linkish',
          i18n: 'wsdl.list.clearAll',
          onclick: () => withBusyAll(() => clearJobItems('')),
        })));
    }
    if (job.error === 'login') rows.append(wsdlEl('div', { className: 'warning-bar err-bar', i18n: 'wsdl.loginLost' }));
    if (busy && job.log?.length) rows.append(wsdlEl('pre', { className: 'wsdl-log', text: job.log.join('\n') }));

    nodes.push(rows);
    jobRegion.set(nodes);
  }

  function renderModsStep() {
    const busy = isBusy();
    modsStep.setDone(false);
    lookupButton.disabled = busy;
    updatesButton.disabled = busy;
    renderResults();
    renderJob();
  }

  function render() {
    stepRoots.forEach((root) => wsdlSetVisible(root, true));
    renderToolStep();
    renderLoginStep();
    renderModsStep();
    applyI18n();
  }

  card._sync = () => watch(api.post('/api/workshop/dl/sync', {}));
  load();
  return card;
}