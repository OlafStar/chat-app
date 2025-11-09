const dom = {
  tableList: document.getElementById('table-list'),
  tableEmpty: document.getElementById('table-empty'),
  tableDetails: document.getElementById('table-details'),
  tableName: document.getElementById('table-name'),
  tableMeta: document.getElementById('table-meta'),
  metaGrid: document.getElementById('meta-grid'),
  rawTable: document.getElementById('raw-table'),
  limitInput: document.getElementById('limit-input'),
  reloadItems: document.getElementById('reload-items'),
  nextPage: document.getElementById('next-page'),
  refreshTables: document.getElementById('refresh-tables'),
  envPill: document.getElementById('env-pill'),
  itemsHead: document.getElementById('items-head'),
  itemsBody: document.getElementById('items-body'),
  tableFooter: document.getElementById('table-footer'),
};

const state = {
  tables: [],
  selectedTable: null,
  nextToken: '',
  limit: 25,
};

init();

function init() {
  dom.refreshTables.addEventListener('click', loadTables);
  dom.reloadItems.addEventListener('click', () => loadItems(''));
  dom.nextPage.addEventListener('click', () => {
    if (state.nextToken) {
      loadItems(state.nextToken);
    }
  });
  dom.limitInput.addEventListener('change', () => {
    const value = Number(dom.limitInput.value);
    if (!Number.isFinite(value) || value <= 0) {
      dom.limitInput.value = state.limit;
      return;
    }
    state.limit = value;
  });

  Promise.all([loadConfig(), loadTables()]).catch((error) => {
    console.error(error);
    showToast('Failed to boot viewer. Check console for details.');
  });
}

async function loadConfig() {
  const config = await fetchJSON('/api/config');
  state.limit = config.defaultLimit;
  dom.limitInput.value = config.defaultLimit;
  dom.envPill.textContent = `${config.region} · ${config.endpoint || 'aws.com'}`;
}

async function loadTables() {
  dom.tableList.innerHTML = '<p class="muted">Loading…</p>';
  try {
    const data = await fetchJSON('/api/tables');
    state.tables = (data.tables || []).slice().sort();
    renderTableList();
  } catch (error) {
    console.error(error);
    dom.tableList.innerHTML = '<p class="error">Unable to load tables.</p>';
  }
}

function renderTableList() {
  if (!state.tables.length) {
    dom.tableList.innerHTML = '<p class="muted">No tables detected.</p>';
    return;
  }

  dom.tableList.innerHTML = '';
  state.tables.forEach((table) => {
    const btn = document.createElement('button');
    btn.className = 'table-button' + (state.selectedTable === table ? ' active' : '');
    btn.textContent = table;
    btn.addEventListener('click', () => selectTable(table));
    dom.tableList.appendChild(btn);
  });
}

async function selectTable(table) {
  state.selectedTable = table;
  state.nextToken = '';
  renderTableList();
  dom.tableEmpty.classList.add('hidden');
  dom.tableDetails.classList.remove('hidden');
  dom.tableName.textContent = table;
  dom.tableMeta.textContent = 'Fetching metadata…';
  dom.metaGrid.innerHTML = '';
  dom.rawTable.textContent = '';
  dom.itemsHead.innerHTML = '';
  dom.itemsBody.innerHTML = '<tr><td>Loading…</td></tr>';
  dom.tableFooter.textContent = '';
  dom.nextPage.disabled = true;

  await Promise.all([loadMeta(table), loadItems('')]);
}

async function loadMeta(table) {
  try {
    const data = await fetchJSON(`/api/tables/${encodeURIComponent(table)}/meta`);
    const meta = data.table;
    dom.tableMeta.textContent = `${meta.TableStatus} · ${meta.ItemCount ?? 0} items · ${(meta.TableSizeBytes ?? 0).toLocaleString()} bytes`;
    dom.metaGrid.innerHTML = '';
    addMetaCard('Partition key', keySchema(meta, 'HASH'));
    addMetaCard('Sort key', keySchema(meta, 'RANGE'));
    addMetaCard('Billing mode', meta.BillingModeSummary?.BillingMode || 'PROVISIONED');
    addMetaCard('Stream', meta.StreamSpecification?.StreamEnabled ? meta.StreamSpecification.StreamViewType : 'Disabled');
    addMetaCard('GSIs', (meta.GlobalSecondaryIndexes || []).length || '—');
    addMetaCard('LSIs', (meta.LocalSecondaryIndexes || []).length || '—');
    dom.rawTable.textContent = JSON.stringify(meta, null, 2);
  } catch (error) {
    console.error(error);
    dom.tableMeta.textContent = 'Unable to load metadata';
  }
}

function keySchema(meta, keyType) {
  const found = (meta.KeySchema || []).find((k) => k.KeyType === keyType);
  return found ? `${found.AttributeName}` : '—';
}

function addMetaCard(label, value) {
  const card = document.createElement('div');
  card.className = 'meta-card';
  card.innerHTML = `<span>${label}</span><strong>${value ?? '—'}</strong>`;
  dom.metaGrid.appendChild(card);
}

async function loadItems(token) {
  if (!state.selectedTable) return;
  try {
    const params = new URLSearchParams({ limit: String(state.limit) });
    if (token) {
      params.set('startKey', token);
    }

    const data = await fetchJSON(`/api/tables/${encodeURIComponent(state.selectedTable)}/items?${params}`);
    state.nextToken = data.nextPageToken || '';
    dom.nextPage.disabled = !state.nextToken;

    renderItems(data.items || []);
    dom.tableFooter.innerHTML = `
      <span>${data.items?.length || 0} items · count=${data.count} · scanned=${data.scannedCount}</span>
      <span>${state.nextToken ? 'More rows available' : 'End of table slice'}</span>
    `;
  } catch (error) {
    console.error(error);
    dom.itemsHead.innerHTML = '';
    dom.itemsBody.innerHTML = '<tr><td>Error loading data.</td></tr>';
    dom.tableFooter.textContent = '';
  }
}

function renderItems(items) {
  if (!items.length) {
    dom.itemsHead.innerHTML = '';
    dom.itemsBody.innerHTML = '<tr><td>No rows returned.</td></tr>';
    return;
  }

  const columns = Array.from(
    items.reduce((acc, item) => {
      Object.keys(item).forEach((key) => acc.add(key));
      return acc;
    }, new Set())
  ).sort();

  dom.itemsHead.innerHTML = `<tr>${columns.map((col) => `<th>${col}</th>`).join('')}</tr>`;
  dom.itemsBody.innerHTML = items
    .map((item) => {
      const cells = columns
        .map((col) => `<td><code>${formatCell(item[col])}</code></td>`)
        .join('');
      return `<tr>${cells}</tr>`;
    })
    .join('');
}

function formatCell(value) {
  if (value === null || value === undefined) return '';
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
    return escapeHtml(String(value));
  }
  return escapeHtml(JSON.stringify(value));
}

function escapeHtml(value) {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

async function fetchJSON(url, options) {
  const res = await fetch(url, options);
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Request failed with ${res.status}`);
  }
  return res.json();
}

function showToast(message) {
  alert(message);
}
