const health = document.querySelector('#health');
const ready = document.querySelector('#ready');
const rollouts = document.querySelector('#rollouts');
const count = document.querySelector('#count');

async function probe(path, target) {
  try {
    const response = await fetch(path);
    target.textContent = response.ok ? '正常' : `异常 ${response.status}`;
    target.className = response.ok ? 'ok' : 'bad';
  } catch {
    target.textContent = '无法连接';
    target.className = 'bad';
  }
}

async function loadRollouts() {
  try {
    const response = await fetch('/v1/rollouts?limit=8');
    const body = await response.json();
    const items = body.data || [];
    count.textContent = `${items.length} 条`;
    rollouts.innerHTML = items.length ? items.map(item => `
      <article class="rollout">
        <strong>${item.name || item.id}</strong><span>${item.status}</span>
        <p>${item.strategy || '未指定策略'}</p><p>${item.updated_at || ''}</p>
      </article>`).join('') : '<p class="empty">当前没有发布任务。</p>';
  } catch {
    rollouts.innerHTML = '<p class="empty">发布任务暂时读取失败。</p>';
  }
}

async function refresh() {
  await Promise.all([probe('/healthz', health), probe('/readyz', ready), loadRollouts()]);
}

document.querySelector('#refresh').addEventListener('click', refresh);
refresh();
