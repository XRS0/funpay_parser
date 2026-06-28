import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { createRoot } from 'react-dom/client';
import {
  Bot,
  CheckCircle2,
  Clock,
  Database,
  Edit3,
  ExternalLink,
  Play,
  Plus,
  Save,
  Search,
  KeyRound,
  Route,
  Settings as SettingsIcon,
  ShieldCheck,
  SlidersHorizontal,
  Wifi,
  Square,
  Trash2,
  X,
} from 'lucide-react';
import './style.css';

const defaultModels = {
  fireworks: 'accounts/fireworks/models/llama-v3p1-70b-instruct',
  openrouter: 'openai/gpt-4o-mini',
};

const presets = ['chatgpt plus', 'chatgpt team', 'midjourney', 'netflix'];

async function api(path, options = {}) {
  const resp = await fetch(path, {
    ...options,
    headers: {
      ...(options.body ? { 'Content-Type': 'application/json' } : {}),
      ...(options.headers || {}),
    },
  });
  const text = await resp.text();
  const data = text ? JSON.parse(text) : null;
  if (!resp.ok) {
    throw new Error(data?.error || `HTTP ${resp.status}`);
  }
  return data;
}

function useToast() {
  const [toast, setToast] = useState(null);
  const timer = useRef(null);
  const showToast = useCallback((message, isError = false) => {
    window.clearTimeout(timer.current);
    setToast({ message, isError, hiding: false });
    timer.current = window.setTimeout(() => {
      setToast((t) => (t ? { ...t, hiding: true } : t));
      window.setTimeout(() => setToast(null), 350);
    }, 3500);
  }, []);
  useEffect(() => () => window.clearTimeout(timer.current), []);
  return [toast, showToast];
}

function formatDate(value, withYear = true) {
  if (!value) return '—';
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return '—';
  return d.toLocaleString('ru-RU', {
    day: '2-digit',
    month: '2-digit',
    ...(withYear ? { year: 'numeric' } : {}),
    hour: '2-digit',
    minute: '2-digit',
  });
}

function priceText(item) {
  if (!item || item.price == null) return '—';
  return `${item.price} ${item.currency || ''}`.trim();
}

function typeLabel(value) {
  if (value === 'personal') return 'Личный';
  if (value === 'shared') return 'Общий';
  return '—';
}

function safeList(value) {
  return Array.isArray(value) ? value : [];
}

function currentPath() {
  const p = window.location.pathname;
  return ['/', '/saved', '/scheduler', '/settings'].includes(p) ? p : '/';
}

function navigate(path) {
  window.history.pushState({}, '', path);
  window.dispatchEvent(new Event('popstate'));
}

function Background() {
  const canvasRef = useRef(null);

  useEffect(() => {
    let mouseX = 0;
    let mouseY = 0;
    let scrollY = window.scrollY;
    let smoothMouseX = 0;
    let smoothMouseY = 0;
    let smoothScrollY = scrollY;
    const lerpFactor = 0.04;
    let animation = 0;
    let lastFrame = 0;
    let width = window.innerWidth;
    let height = window.innerHeight;
    let stars = [];
    let shootingStars = [];
    const canvas = canvasRef.current;
    const ctx = canvas?.getContext?.('2d');
    if (!canvas || !ctx) return undefined;

    const onMove = (e) => {
      mouseX = (e.clientX / window.innerWidth - 0.5) * 2;
      mouseY = (e.clientY / window.innerHeight - 0.5) * 2;
    };
    const onScroll = () => {
      scrollY = window.scrollY;
    };

    function smoothInput() {
      smoothMouseX += (mouseX - smoothMouseX) * lerpFactor;
      smoothMouseY += (mouseY - smoothMouseY) * lerpFactor;
      smoothScrollY += (scrollY - smoothScrollY) * lerpFactor;
      document.querySelectorAll('.parallax').forEach((el) => {
        const speed = Number.parseFloat(el.dataset.speed || '0.03');
        el.style.transform = `translate(${smoothMouseX * speed * 30}px, ${smoothMouseY * speed * 30 + smoothScrollY * speed * 0.2}px)`;
      });
    }

    function resize() {
      width = window.innerWidth;
      height = window.innerHeight;
      const dpr = Math.min(window.devicePixelRatio || 1, 2);
      canvas.width = Math.floor(width * dpr);
      canvas.height = Math.floor(height * dpr);
      ctx.setTransform(1, 0, 0, 1, 0, 0);
      ctx.scale(dpr, dpr);
      const count = Math.min(700, Math.floor((width * height) / 3500));
      stars = Array.from({ length: count }, () => {
        const depth = Math.random() * 0.8 + 0.2;
        const isBig = Math.random() < 0.12;
        const far = depth < 0.35;
        return {
          baseX: Math.random() * width,
          baseY: Math.random() * height,
          radius: isBig ? Math.random() * 1.6 + 1.2 : far ? Math.random() * 0.7 + 0.2 : Math.random() * 1.1 + 0.4,
          baseAlpha: isBig ? Math.random() * 0.2 + 0.75 : far ? Math.random() * 0.25 + 0.3 : Math.random() * 0.35 + 0.55,
          twinkleSpeed: Math.random() * 0.05 + 0.005,
          twinklePhase: Math.random() * Math.PI * 2,
          depth,
        };
      });
    }

    function drawStar(star) {
      const x = (star.baseX + smoothMouseX * star.depth * 20 + width) % width;
      const y = (star.baseY + smoothMouseY * star.depth * 20 + smoothScrollY * star.depth * 0.2 + height) % height;
      star.twinklePhase += star.twinkleSpeed;
      const alpha = Math.max(0.05, Math.min(1, star.baseAlpha + Math.sin(star.twinklePhase) * 0.2));
      ctx.beginPath();
      ctx.arc(x, y, star.radius * 3, 0, Math.PI * 2);
      ctx.fillStyle = `rgba(255,255,255,${alpha * 0.1})`;
      ctx.fill();
      ctx.beginPath();
      ctx.arc(x, y, star.radius, 0, Math.PI * 2);
      ctx.fillStyle = `rgba(255,255,255,${alpha})`;
      ctx.fill();
    }

    function spawnShootingStar() {
      if (Math.random() > 0.008) return;
      const speed = Math.random() * 10 + 3;
      shootingStars.push({ x: Math.random() * width * 0.5, y: Math.random() * height * 0.5, vx: speed, vy: speed * 0.35, length: Math.random() * 90 + 60, life: 1, decay: 0.012 });
    }

    function drawShootingStar(s) {
      const tailX = s.x - s.vx * (s.length / 5);
      const tailY = s.y - s.vy * (s.length / 5);
      const grad = ctx.createLinearGradient(tailX, tailY, s.x, s.y);
      grad.addColorStop(0, 'rgba(255,255,255,0)');
      grad.addColorStop(0.5, `rgba(255,255,255,${s.life * 0.5})`);
      grad.addColorStop(1, `rgba(255,255,255,${s.life})`);
      ctx.strokeStyle = grad;
      ctx.lineWidth = 2;
      ctx.lineCap = 'round';
      ctx.beginPath();
      ctx.moveTo(tailX, tailY);
      ctx.lineTo(s.x, s.y);
      ctx.stroke();
    }

    function animate(timestamp) {
      animation = requestAnimationFrame(animate);
      smoothInput();
      if (timestamp - lastFrame < 16) return;
      lastFrame = timestamp;
      ctx.clearRect(0, 0, width, height);
      stars.forEach(drawStar);
      spawnShootingStar();
      shootingStars.forEach((s, i) => {
        s.x += s.vx;
        s.y += s.vy;
        s.life -= s.decay;
        drawShootingStar(s);
        if (s.life <= 0 || s.x > width + 100 || s.y > height + 100) shootingStars.splice(i, 1);
      });
    }

    resize();
    window.addEventListener('mousemove', onMove);
    window.addEventListener('scroll', onScroll, { passive: true });
    window.addEventListener('resize', resize);
    animation = requestAnimationFrame(animate);
    return () => {
      cancelAnimationFrame(animation);
      window.removeEventListener('mousemove', onMove);
      window.removeEventListener('scroll', onScroll);
      window.removeEventListener('resize', resize);
    };
  }, []);

  return (
    <>
      <div className="bg-shapes" />
      <canvas ref={canvasRef} id="stars" className="stars-canvas" />
      <div className="glow-layer">
        {[0.02, 0.04, 0.03, 0.05, 0.06].map((speed, idx) => (
          <div key={speed} className="parallax" data-speed={speed}>
            <div className={`shape shape-${idx + 1}`} />
          </div>
        ))}
      </div>
    </>
  );
}

function Toast({ toast }) {
  if (!toast) return null;
  return <div className={`toast ${toast.isError ? ' error' : ''} visible ${toast.hiding ? 'hiding' : ''}`}>{toast.message}</div>;
}

function Brand({ title, subtitle }) {
  return (
    <div className="brand">
      <div className="brand-icon">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
          <circle cx="12" cy="12" r="5" />
          <ellipse cx="12" cy="12" rx="9" ry="3" transform="rotate(-35 12 12)" />
          <circle cx="19" cy="8" r="1.5" fill="currentColor" stroke="none" />
        </svg>
      </div>
      <div>
        <h1>{title}</h1>
        <p className="subtitle">{subtitle}</p>
      </div>
    </div>
  );
}

function NavButton({ to, children, icon }) {
  return (
    <button type="button" className="btn btn-secondary btn-sm" onClick={() => navigate(to)}>
      {icon}
      {children}
    </button>
  );
}

function Header({ title = 'Funpay Parser', subtitle = 'мабой' }) {
  const path = currentPath();
  return (
    <header className="app-header">
      <Brand title={title} subtitle={subtitle} />
      <div className="header-actions">
        {path !== '/scheduler' && <NavButton to="/scheduler" icon={<Clock size={18} />}>Расписание</NavButton>}
        {path !== '/saved' && <NavButton to="/saved" icon={<Database size={18} />}>Сохранённые результаты</NavButton>}
        {path !== '/settings' && <NavButton to="/settings" icon={<SettingsIcon size={18} />}>Настройки</NavButton>}
        {path !== '/' && <NavButton to="/" icon={<Play size={18} />}>К парсеру</NavButton>}
      </div>
    </header>
  );
}

function Badge({ children, className = 'neutral' }) {
  return <span className={`badge ${className}`}>{children}</span>;
}

function Field({ label, children }) {
  return (
    <div className="field">
      <label>{label}</label>
      {children}
    </div>
  );
}

function AnimatedMetricValue({ value, pulseKey, className = '' }) {
  const numeric = Number(value) || 0;
  const [display, setDisplay] = useState(numeric);

  useEffect(() => {
    const from = display;
    const to = numeric;
    if (from === to) return undefined;
    const start = performance.now();
    const duration = 520;
    let frame = 0;
    const tick = (now) => {
      const t = Math.min(1, (now - start) / duration);
      const eased = 1 - Math.pow(1 - t, 3);
      setDisplay(Math.round(from + (to - from) * eased));
      if (t < 1) frame = requestAnimationFrame(tick);
    };
    frame = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(frame);
  }, [numeric]);

  return <span key={pulseKey} className={`metric-value metric-value-animated ${className}`}>{display}</span>;
}

function StatusPipeline({ progress }) {
  const text = progress.map((p) => p.message || '').join('\n').toLowerCase();
  const steps = [
    ['fetch', 'Сбор', Search],
    ['filter', 'Фильтр', Database],
    ['preselect', 'Отбор', Plus],
    ['classify', 'LLM', SettingsIcon],
    ['result', 'Результат', Save],
  ];
  const activeIndex = text.includes('classifying') || text.includes('llm') ? 3 : text.includes('limited') || text.includes('candidate') ? 2 : text.includes('filtered') ? 1 : text.includes('found') || text.includes('fetch') ? 0 : -1;
  const progressPct = Math.max(8, ((activeIndex + 1) / steps.length) * 100);
  const metrics = useMemo(() => {
    const messages = progress.map((p) => p.message || '');
    const joined = messages.join(' ');
    const llmDone = messages.filter((m) => /^\[LLM\]/.test(m.trim())).length;
    const candidates =
      joined.match(/Limited LLM candidates to (\d+)/)?.[1]
      || joined.match(/Sending (\d+) filtered listings to LLM/)?.[1]
      || '0';
    const summaryClassified = joined.match(/classified[\s\":]+(\d+)/i)?.[1];
    return {
      listings: Number(joined.match(/Found (\d+) listings/)?.[1] || 0),
      candidates: Number(candidates),
      classified: Number(summaryClassified || llmDone || 0),
      llmDone,
    };
  }, [progress]);
  const llmPercent = metrics.candidates > 0 ? Math.min(100, Math.round((metrics.classified / metrics.candidates) * 100)) : 0;
  const llmActive = activeIndex >= 3 && metrics.candidates > 0 && metrics.classified < metrics.candidates;

  return (
    <div className="status-loader">
      <div className="status-dashboard">
        <div className="status-metrics">
          <div className="metric"><AnimatedMetricValue value={metrics.listings} pulseKey={`listings-${metrics.listings}`} /><span className="metric-label">Лотов</span></div>
          <div className="metric"><AnimatedMetricValue value={metrics.candidates} pulseKey={`candidates-${metrics.candidates}`} /><span className="metric-label">Кандидатов</span></div>
          <div className={`metric metric-llm ${llmActive ? 'active live' : metrics.classified > 0 ? 'done' : ''}`} style={{ '--llm-progress': `${llmPercent}%` }}><AnimatedMetricValue value={metrics.classified} pulseKey={`classified-${metrics.classified}`} className="metric-value-llm" /><span className="metric-label">Проверено LLM</span><span className="metric-subline">{metrics.candidates ? `${llmPercent}% из ${metrics.candidates}` : 'ожидание'}</span></div>
        </div>
        <div className="status-pipeline">
          {steps.map(([key, label, Icon], idx) => (
            <React.Fragment key={key}>
              <div className={`pipeline-step ${idx <= activeIndex ? 'active' : ''} ${idx < activeIndex ? 'done' : ''}`} data-step={key}>
                <div className="step-ring"><Icon className="step-icon" size={18} /><div className="step-spinner" /></div>
                <div className="step-label">{label}</div>
              </div>
              {idx < steps.length - 1 && <div className="pipeline-connector" />}
            </React.Fragment>
          ))}
        </div>
        <div className="progress-track"><div className="progress-fill" style={{ width: `${progressPct}%` }} /></div>
      </div>
    </div>
  );
}

function ResultsView({ status, selectedProfileId, showToast }) {
  const cheapest = status.cheapest;
  const summary = status.result_summary || {};
  const [saving, setSaving] = useState(false);
  const [full, setFull] = useState(null);

  const loadFull = useCallback(async () => {
    try {
      setFull(await api('/results'));
    } catch {
      setFull(null);
    }
  }, []);

  useEffect(() => {
    if (cheapest) loadFull();
  }, [cheapest, loadFull]);

  const rows = safeList(full?.all_results || full?.listings).filter((r) => r.is_plus).slice(0, 80);

  const saveResult = async () => {
    if (!selectedProfileId || !full) return;
    setSaving(true);
    try {
      await api('/api/saved_results', { method: 'POST', body: JSON.stringify({ profile_id: selectedProfileId, cheapest: full.cheapest, summary: full.summary, all_results: full.all_results }) });
      showToast('Результат сохранён');
    } catch (err) {
      showToast(err.message, true);
    } finally {
      setSaving(false);
    }
  };

  if (!cheapest && !summary) return null;
  return (
    <section className="section reveal visible" id="result-section">
      <div className="section-header">
        <div className="section-label">Результаты</div>
        {selectedProfileId && full && <button className="btn btn-success btn-sm" disabled={saving} onClick={saveResult}><Save size={18} />Сохранить в историю</button>}
      </div>
      {cheapest && (
        <div className="card cheapest-card">
          <div className="cheapest-hero">
            <div className="cheapest-header"><span className="cheapest-badge">Самый дешёвый личный</span><Badge className={cheapest.account_type || 'neutral'}>{typeLabel(cheapest.account_type)}</Badge></div>
            <div className="cheapest-price">{cheapest.price ?? '—'}<span className="cheapest-currency">{cheapest.currency || ''}</span></div>
            <div className="cheapest-title">{cheapest.title || '—'}</div>
            <div className="cheapest-meta"><span>{cheapest.seller || '—'}</span><span>confidence: {cheapest.confidence?.toFixed?.(2) || '—'}</span></div>
          </div>
          <a className="btn btn-primary cheapest-link" href={cheapest.url || '#'} target="_blank" rel="noreferrer">Открыть на Funpay <ExternalLink size={18} /></a>
        </div>
      )}
      <div className="card summary-card" style={{ marginBottom: 20 }}>
        <div className="section-label" style={{ marginTop: 0 }}>Сводка по результатам</div>
        <div className="summary-grid">
          {Object.entries(summary).map(([k, v]) => <div className="summary-item" key={k}><span className="summary-value">{v}</span><span className="summary-label">{k}</span></div>)}
        </div>
      </div>
      {!!rows.length && (
        <div className="table-wrap">
          <table id="results-table">
            <thead><tr><th>Цена</th><th>Тип</th><th>Уверенность</th><th>Заголовок</th><th>Продавец</th><th>Ссылка</th></tr></thead>
            <tbody>{rows.map((r, idx) => <tr key={`${r.url || idx}`} className={r.account_type || ''}><td>{priceText(r)}</td><td><Badge className={r.account_type || 'neutral'}>{typeLabel(r.account_type)}</Badge></td><td>{r.confidence?.toFixed?.(2) || '—'}</td><td>{r.title || ''}</td><td>{r.seller || ''}</td><td><a href={r.url || '#'} target="_blank" rel="noreferrer">Funpay ↗</a></td></tr>)}</tbody>
          </table>
        </div>
      )}
    </section>
  );
}

function HomePage({ showToast }) {
  const [profiles, setProfiles] = useState([]);
  const [selectedProfile, setSelectedProfile] = useState(null);
  const [profileModal, setProfileModal] = useState(null);
  const [query, setQuery] = useState('chatgpt plus');
  const [categoryID, setCategoryID] = useState(1355);
  const [candidates, setCandidates] = useState(40);
  const [pages, setPages] = useState('');
  const [deep, setDeep] = useState(false);
  const [status, setStatus] = useState({ running: false, status: 'idle', progress: [] });
  const [hasLiveStatus, setHasLiveStatus] = useState(false);

  const loadProfiles = useCallback(async () => {
    setProfiles(await api('/api/profiles'));
  }, []);
  const pollStatus = useCallback(async () => {
    try {
      const next = await api('/status');
      if (next.running) setHasLiveStatus(true);
      setStatus(next);
    } catch { /* ignore */ }
  }, []);

  useEffect(() => {
    loadProfiles().catch((err) => showToast(err.message, true));
    pollStatus();
    const id = window.setInterval(pollStatus, 1500);
    return () => window.clearInterval(id);
  }, [loadProfiles, pollStatus, showToast]);

  const applyProfile = (p) => {
    if (selectedProfile?.id === p.id) {
      setSelectedProfile(null);
      return;
    }
    setSelectedProfile(p);
    setQuery(p.query || 'chatgpt plus');
    setCategoryID(p.category_id || 1355);
    setCandidates(p.candidates || 40);
    setPages(p.max_pages || '');
    setDeep(!!p.deep);
  };

  const run = async () => {
    try {
      setHasLiveStatus(true);
      await api('/run', { method: 'POST', body: JSON.stringify(selectedProfile ? { profile_id: selectedProfile.id } : { query, category_id: Number(categoryID), candidates: Number(candidates), pages: pages ? Number(pages) : 0, deep }) });
      showToast('Парсер запущен');
      await pollStatus();
    } catch (err) { showToast(err.message, true); }
  };
  const stop = async () => {
    try { await api('/stop', { method: 'POST' }); showToast('Останавливаю'); await pollStatus(); } catch (err) { showToast(err.message, true); }
  };
  const saveCurrentToProfile = async () => {
    if (!selectedProfile) return;
    try {
      const updated = await api(`/api/profiles/${selectedProfile.id}`, { method: 'PUT', body: JSON.stringify({ ...selectedProfile, query, category_id: Number(categoryID), candidates: Number(candidates), max_pages: pages ? Number(pages) : null, deep }) });
      setSelectedProfile(updated);
      await loadProfiles();
      showToast('Профиль обновлён');
    } catch (err) { showToast(err.message, true); }
  };
  const deleteProfile = async (p) => {
    if (!window.confirm(`Удалить профиль «${p.name}»?`)) return;
    try { await api(`/api/profiles/${p.id}`, { method: 'DELETE' }); if (selectedProfile?.id === p.id) setSelectedProfile(null); await loadProfiles(); showToast('Профиль удалён'); } catch (err) { showToast(err.message, true); }
  };
  const showStatusBlock = status.running || (hasLiveStatus && safeList(status.progress).length > 0);

  return (
    <>
      <Header />
      <main className="main">
        <section className="section reveal visible search-profiles-section compact-profiles">
          <div className="section-header profiles-header-compact">
            <div className="section-label">Профили поиска</div>
            <button className="btn btn-primary btn-sm profiles-create" onClick={() => setProfileModal({})}><Plus size={18} />Новый профиль</button>
          </div>
          {!profiles.length ? (
            <div className="profiles-empty-compact">
              <span>Профилей нет</span>
              <button className="btn btn-secondary btn-sm" onClick={() => setProfileModal({})}>Создать</button>
            </div>
          ) : (
            <div className="profiles-list compact-list stagger visible">
              {profiles.map((p, i) => <button type="button" key={p.id} className={`profile-card compact-profile-card stagger-item ${selectedProfile?.id === p.id ? 'active' : ''}`} style={{ animationDelay: `${i * 0.025}s` }} onClick={() => applyProfile(p)}>
                <div className="profile-main-line">
                  <span className="profile-name">{p.name}</span>
                  <span className="profile-query">{p.query}</span>
                </div>
                <div className="profile-meta-row compact">
                  <span>ID {p.category_id}</span>
                  <span>{p.candidates} канд.</span>
                  <span>{p.max_pages ? `${p.max_pages} стр.` : 'все'}</span>
                  {p.deep && <span>Deep</span>}
                </div>
                <div className="profile-actions compact-actions" aria-label="Действия профиля">
                  <span className="profile-action-link" onClick={(e) => { e.stopPropagation(); setProfileModal(p); }}><Edit3 size={14} /></span>
                  <span className="profile-action-link danger" onClick={(e) => { e.stopPropagation(); deleteProfile(p); }}><Trash2 size={14} /></span>
                </div>
              </button>)}
            </div>
          )}
        </section>

        <section className="section reveal visible">
          <div className="section-header"><div className="section-label">Текущий запуск</div><Badge>{selectedProfile ? selectedProfile.name : 'Без профиля'}</Badge></div>
          <div className="card run-card">
            <label className="run-label">Поисковый запрос</label>
            <div className="run-command">
              <div className="input-icon run-input"><Search size={20} /><input value={query} onChange={(e) => { setQuery(e.target.value); setSelectedProfile(null); }} placeholder="например, chatgpt plus" /></div>
              {!status.running ? <button className="btn btn-primary btn-lg run-btn" onClick={run}><Play size={20} /><span>Запустить</span></button> : <button className="btn btn-danger btn-lg stop-btn" onClick={stop}><Square size={20} /><span>Остановить</span></button>}
            </div>
            <div className="query-presets">{presets.map((p) => <button key={p} type="button" className="preset-btn" onClick={() => { setQuery(p); setSelectedProfile(null); }}>{p.replace(/\b\w/g, (m) => m.toUpperCase())}</button>)}</div>
            <div className="run-settings">
              <Field label="Category ID"><input type="number" value={categoryID} min="1" onChange={(e) => { setCategoryID(e.target.value); setSelectedProfile(null); }} /></Field>
              <Field label="Кандидатов"><input type="number" value={candidates} min="1" max="200" onChange={(e) => { setCandidates(e.target.value); setSelectedProfile(null); }} /></Field>
              <Field label="Страниц"><input type="number" value={pages} min="1" placeholder="все" onChange={(e) => { setPages(e.target.value); setSelectedProfile(null); }} /></Field>
            </div>
            <div className="run-options"><label className="toggle-switch"><input type="checkbox" checked={deep} onChange={(e) => { setDeep(e.target.checked); setSelectedProfile(null); }} /><span className="toggle-slider" /><span className="toggle-label">Deep mode</span></label>{selectedProfile && <button className="btn btn-secondary btn-sm" onClick={saveCurrentToProfile}>Сохранить в профиль</button>}</div>
          </div>
        </section>

        {showStatusBlock && <section className="section reveal visible"><div className="section-header"><div className="section-label">Статус</div><div className={`status-badge ${status.running ? 'active' : status.status === 'Done' ? 'done' : 'idle'}`}><span className="status-dot" /><span className="status-text">{status.status || 'Ожидание'}</span></div></div><div className="card status-card"><StatusPipeline progress={safeList(status.progress)} /><div className="progress-terminal"><div>{safeList(status.progress).map((p, i) => <div key={`${p.time}-${i}`} className="progress-line"><span className="time">{p.time}</span> <span>{p.message}</span></div>)}</div>{status.running && <div className="progress-cursor"><span className="cursor" /></div>}</div></div></section>}
        <ResultsView status={status} selectedProfileId={selectedProfile?.id || status.profile_id} showToast={showToast} />
      </main>
      {profileModal !== null && <ProfileModal profile={profileModal} onClose={() => setProfileModal(null)} onSaved={async (p) => { await loadProfiles(); setProfileModal(null); applyProfile(p); showToast('Профиль сохранён'); }} showToast={showToast} />}
    </>
  );
}

function ProfileModal({ profile, onClose, onSaved, showToast }) {
  const [form, setForm] = useState({ name: profile.name || '', query: profile.query || 'chatgpt plus', category_id: profile.category_id || 1355, candidates: profile.candidates || 40, max_pages: profile.max_pages || '', deep: !!profile.deep });
  const submit = async () => {
    try {
      const body = { ...form, category_id: Number(form.category_id), candidates: Number(form.candidates), max_pages: form.max_pages ? Number(form.max_pages) : null };
      const saved = profile.id ? await api(`/api/profiles/${profile.id}`, { method: 'PUT', body: JSON.stringify(body) }) : await api('/api/profiles', { method: 'POST', body: JSON.stringify(body) });
      onSaved(saved);
    } catch (err) { showToast(err.message, true); }
  };
  return <Modal title={profile.id ? 'Редактировать профиль' : 'Новый профиль'} onClose={onClose} footer={<><button className="btn btn-secondary" onClick={onClose}>Отмена</button><button className="btn btn-primary" onClick={submit}>Сохранить</button></>}>
    <div className="form-grid"><Field label="Название"><input className="form-input" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} /></Field><Field label="Запрос"><input className="form-input" value={form.query} onChange={(e) => setForm({ ...form, query: e.target.value })} /></Field><Field label="Category ID"><input className="form-input" type="number" value={form.category_id} onChange={(e) => setForm({ ...form, category_id: e.target.value })} /></Field><Field label="Кандидатов"><input className="form-input" type="number" value={form.candidates} onChange={(e) => setForm({ ...form, candidates: e.target.value })} /></Field><Field label="Страниц"><input className="form-input" type="number" value={form.max_pages} onChange={(e) => setForm({ ...form, max_pages: e.target.value })} /></Field></div>
    <div style={{ marginTop: 16 }}><label className="toggle-switch"><input type="checkbox" checked={form.deep} onChange={(e) => setForm({ ...form, deep: e.target.checked })} /><span className="toggle-slider" /><span className="toggle-label">Deep mode</span></label></div>
  </Modal>;
}

function Modal({ title, children, footer, onClose, className = '' }) {
  useEffect(() => {
    const handleKeyDown = (event) => {
      if (event.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [onClose]);

  return <div className="modal visible"><div className="modal-overlay" onClick={onClose} /><div className={`modal-content ${className}`}><div className="modal-header"><h2 className="modal-title">{title}</h2><button className="modal-close" onClick={onClose}><X size={20} /></button></div><div className="modal-body">{children}</div><div className="modal-footer">{footer}</div></div></div>;
}

function SavedPage({ showToast }) {
  const [profiles, setProfiles] = useState([]);
  const [saved, setSaved] = useState([]);
  const [detail, setDetail] = useState(null);
  const load = useCallback(async () => {
    const [p, s] = await Promise.all([api('/api/profiles'), api('/api/saved_results')]);
    setProfiles(p); setSaved(s);
  }, []);
  useEffect(() => { load().catch((err) => showToast(err.message, true)); }, [load, showToast]);
  const profileName = (id) => profiles.find((p) => p.id === id)?.name || `Профиль #${id}`;
  const del = async (id) => { if (!window.confirm('Удалить результат?')) return; try { await api(`/api/saved_results/${id}`, { method: 'DELETE' }); await load(); showToast('Результат удалён'); } catch (err) { showToast(err.message, true); } };
  const open = async (id) => { try { setDetail(await api(`/api/saved_results/${id}`)); } catch (err) { showToast(err.message, true); } };
  return <><Header title="Сохранённые результаты" subtitle="История парсинга и детали сохранённых запусков" /><main className="main"><section className="section reveal visible"><div className="section-header"><div className="section-label">История запусков</div><button className="btn btn-ghost btn-sm" onClick={load}>Обновить</button></div>{!saved.length && <div className="empty-state"><div className="empty-title">Нет сохранённых результатов</div><div className="empty-text">Запусти парсер с профилем на главной странице, и результат появится здесь.</div></div>}<div className="saved-grid stagger visible">{saved.map((r, i) => <div className="saved-card stagger-item" key={r.id} style={{ animationDelay: `${i * 0.05}s` }} onClick={() => open(r.id)}><div className="saved-card-main"><div className="saved-date"><Clock size={18} /><span>{formatDate(r.run_at)}</span></div><div className="saved-profile"><Badge className="plan">{profileName(r.profile_id)}</Badge></div><div className="saved-price">{priceText(r.cheapest)}</div><div className="saved-summary"><Badge>{r.summary?.total_plus || 0} Plus</Badge><Badge>{r.summary?.classified || 0} LLM</Badge><Badge className="personal">{r.summary?.personal || 0} личных</Badge><Badge className="shared">{r.summary?.shared || 0} общих</Badge></div></div><div className="saved-actions"><button className="btn btn-icon" onClick={(e) => { e.stopPropagation(); open(r.id); }}><Edit3 size={18} /></button><button className="btn btn-icon" onClick={(e) => { e.stopPropagation(); del(r.id); }}><Trash2 size={18} /></button></div></div>)}</div></section></main>{detail && <SavedDetail data={detail} onClose={() => setDetail(null)} onDelete={async () => { await del(detail.id); setDetail(null); }} />}</>;
}

function SavedDetail({ data, onClose, onDelete }) {
  const cheapest = data.cheapest || {};
  const allPlus = safeList(data.all_results).filter((r) => r.is_plus).sort((a, b) => (a.price || 0) - (b.price || 0)).slice(0, 50);
  return <Modal title="Детали сохранённого результата" className="modal-wide" onClose={onClose} footer={<><button className="btn btn-danger" onClick={onDelete}>Удалить</button><button className="btn btn-secondary" onClick={onClose}>Закрыть</button></>}><div className="cheapest-card card" style={{ marginBottom: 20 }}><div className="cheapest-hero"><div className="cheapest-header"><span className="cheapest-badge">Самый дешёвый личный</span><Badge className={cheapest.account_type || 'neutral'}>{typeLabel(cheapest.account_type)}</Badge></div><div className="cheapest-price">{cheapest.price ?? '—'}<span className="cheapest-currency">{cheapest.currency || ''}</span></div><div className="cheapest-title">{cheapest.title || '—'}</div></div></div><div className="table-wrap"><table><thead><tr><th>Цена</th><th>Тип</th><th>Уверенность</th><th>Заголовок</th><th>Продавец</th><th>Ссылка</th></tr></thead><tbody>{allPlus.map((r, i) => <tr key={`${r.url || i}`} className={r.account_type || ''}><td>{priceText(r)}</td><td><Badge className={r.account_type || 'neutral'}>{typeLabel(r.account_type)}</Badge></td><td>{r.confidence?.toFixed?.(2) || '—'}</td><td>{r.title || ''}</td><td>{r.seller || ''}</td><td><a href={r.url || '#'} target="_blank" rel="noreferrer">Funpay ↗</a></td></tr>)}</tbody></table></div></Modal>;
}

function SchedulerPage({ showToast }) {
  const [profiles, setProfiles] = useState([]);
  const [schedules, setSchedules] = useState([]);
  const [profileID, setProfileID] = useState('');
  const [interval, setIntervalValue] = useState(60);
  const load = useCallback(async () => {
    const [p, s] = await Promise.all([api('/api/profiles'), api('/api/schedules')]);
    setProfiles(p); setSchedules(s); setProfileID((old) => old || String(p[0]?.id || ''));
  }, []);
  useEffect(() => { load().catch((err) => showToast(err.message, true)); }, [load, showToast]);
  const add = async () => { try { await api('/api/schedules', { method: 'POST', body: JSON.stringify({ profile_id: Number(profileID), interval_minutes: Number(interval), enabled: true }) }); await load(); showToast('Расписание добавлено'); } catch (err) { showToast(err.message, true); } };
  const toggle = async (id, enabled) => { try { await api(`/api/schedules/${id}`, { method: 'PUT', body: JSON.stringify({ enabled }) }); await load(); showToast(enabled ? 'Расписание активировано' : 'Расписание остановлено'); } catch (err) { showToast(err.message, true); } };
  const runNow = async (id) => { try { await api(`/api/schedules/${id}/run`, { method: 'POST' }); showToast('Запуск по расписанию начат'); } catch (err) { showToast(err.message, true); } };
  const del = async (id) => { if (!window.confirm('Удалить расписание?')) return; try { await api(`/api/schedules/${id}`, { method: 'DELETE' }); await load(); showToast('Расписание удалено'); } catch (err) { showToast(err.message, true); } };
  return <><Header title="Расписание парсинга" subtitle="Автоматический запуск поиска по расписанию" /><main className="main"><section className="section reveal visible"><div className="section-header"><div className="section-label">Добавить расписание</div></div><div className="card"><div className="form-grid"><Field label="Профиль"><select className="form-input" value={profileID} disabled={!profiles.length} onChange={(e) => setProfileID(e.target.value)}>{profiles.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}</select></Field><Field label="Интервал (минут)"><input className="form-input" type="number" value={interval} min="1" onChange={(e) => setIntervalValue(e.target.value)} /></Field></div><div style={{ marginTop: 16 }}><button className="btn btn-primary" disabled={!profiles.length} onClick={add}>Добавить расписание</button></div></div></section><section className="section reveal visible"><div className="section-header"><div className="section-label">Активные расписания</div><button className="btn btn-ghost btn-sm" onClick={load}>Обновить</button></div>{!schedules.length && <div className="empty-state"><div className="empty-title">Расписаний пока нет</div><div className="empty-text">Выбери профиль и интервал, чтобы парсер запускался автоматически.</div></div>}<div className="saved-grid stagger visible">{schedules.map((s, i) => <div className="saved-card stagger-item" key={s.id} style={{ animationDelay: `${i * 0.05}s` }}><div className="saved-card-main"><div className="saved-date"><Clock size={18} /><span>Интервал: {s.interval_minutes} мин</span></div><div className="saved-profile"><Badge className="plan">{s.profile_name}</Badge></div><div className="saved-summary"><Badge className={s.enabled ? 'success' : 'neutral'}>{s.enabled ? 'Активно' : 'Остановлено'}</Badge><Badge>Следующий: {formatDate(s.next_run_at, false)}</Badge><Badge>Последний: {formatDate(s.last_run_at, false)}</Badge></div></div><div className="saved-actions"><button className="btn btn-icon" onClick={() => runNow(s.id)}><Play size={18} /></button><button className="btn btn-icon" onClick={() => toggle(s.id, !s.enabled)}><Edit3 size={18} /></button><button className="btn btn-icon" onClick={() => del(s.id)}><Trash2 size={18} /></button></div></div>)}</div></section></main></>;
}

function SettingsPage({ showToast }) {
  const [data, setData] = useState(null);
  const [provider, setProvider] = useState('fireworks');
  const [model, setModel] = useState('');
  const [key, setKey] = useState('');
  const [tgToken, setTgToken] = useState('');
  const [tgChat, setTgChat] = useState('');
  const [tgProxy, setTgProxy] = useState('');
  const [funpayProxy, setFunpayProxy] = useState('');
  const [editLLM, setEditLLM] = useState(false);
  const [editTelegram, setEditTelegram] = useState(false);
  const [editFunpay, setEditFunpay] = useState(false);
  const load = useCallback(async () => { const d = await api('/api/settings'); setData(d); setProvider(d.llm_provider || 'fireworks'); setModel(d.llm_model || ''); setTgChat(d.telegram_chat_id || ''); setTgProxy(d.telegram_proxy || ''); setFunpayProxy(d.funpay_proxy || ''); }, []);
  useEffect(() => { load().catch((err) => showToast(err.message, true)); }, [load, showToast]);
  const saveLLM = async () => { if (!key && !window.confirm('Пустое поле удалит текущий ключ. Продолжить?')) return; try { await api('/api/settings', { method: 'PUT', body: JSON.stringify({ llm_provider: provider, llm_model: model.trim(), llm_api_key: key.trim() }) }); setKey(''); setEditLLM(false); await load(); showToast('LLM сохранён'); } catch (err) { showToast(err.message, true); } };
  const saveTelegram = async () => { const body = { telegram_chat_id: tgChat.trim(), telegram_proxy: tgProxy.trim() }; const token = tgToken.trim(); if (token || window.confirm('Пустое поле token удалит текущий token. Продолжить?')) body.telegram_bot_token = token; try { await api('/api/settings', { method: 'PUT', body: JSON.stringify(body) }); setTgToken(''); setEditTelegram(false); await load(); showToast('Telegram сохранён'); } catch (err) { showToast(err.message, true); } };
  const saveFunpay = async () => { try { await api('/api/settings', { method: 'PUT', body: JSON.stringify({ funpay_proxy: funpayProxy.trim() }) }); setEditFunpay(false); await load(); showToast('Funpay proxy сохранён'); } catch (err) { showToast(err.message, true); } };
  const syncTelegram = async () => { try { const d = await api('/api/telegram/sync', { method: 'POST' }); setTgChat(d.chat_id || ''); await load(); showToast('Чат найден'); } catch (err) { showToast(err.message, true); } };
  const testTelegram = async () => { try { await api('/api/telegram/test', { method: 'POST' }); showToast('Тест отправлен'); } catch (err) { showToast(err.message, true); } };
  const llmReady = !!data?.has_key;
  const telegramReady = !!data?.telegram_notifications;
  const proxyReady = !!data?.telegram_proxy_active;
  const funpayProxyReady = !!data?.funpay_proxy_active;
  return <>
    <Header title="Настройки" subtitle="LLM и Telegram" />
    <main className="main settings-page lean-settings">
      <section className="settings-summary-row reveal visible">
        <div className={`settings-pill ${llmReady ? 'ready' : ''}`}><KeyRound size={16} />LLM: {llmReady ? 'готов' : 'нет ключа'}</div>
        <div className={`settings-pill ${telegramReady ? 'ready' : ''}`}><Bot size={16} />Telegram: {telegramReady ? 'включён' : 'выключен'}</div>
        <div className={`settings-pill ${funpayProxyReady ? 'ready' : ''}`}><Route size={16} />Funpay proxy: {funpayProxyReady ? 'активен' : 'нет'}</div>
        <div className={`settings-pill ${proxyReady ? 'ready' : ''}`}><Wifi size={16} />Telegram proxy: {proxyReady ? 'активен' : 'нет'}</div>
      </section>

      <section className="settings-edit-card reveal visible">
        <div className="settings-edit-head">
          <div>
            <h2>LLM</h2>
            <div className="settings-readonly-grid">
              <div><span>Провайдер</span><strong>{data?.llm_provider || provider}</strong></div>
              <div><span>Модель</span><strong>{data?.llm_model || defaultModels[provider]}</strong></div>
              <div><span>Ключ</span><strong>{llmReady ? data.llm_api_key : 'не задан'}</strong></div>
            </div>
          </div>
          <button className="btn btn-secondary btn-sm" onClick={() => setEditLLM((v) => !v)}>{editLLM ? 'Закрыть' : 'Изменить'}</button>
        </div>
        {editLLM && <div className="settings-edit-body">
          <div className="settings-form-grid two">
            <Field label="Провайдер"><select className="form-input clean-input" value={provider} onChange={(e) => setProvider(e.target.value)}><option value="fireworks">Fireworks</option><option value="openrouter">OpenRouter</option></select></Field>
            <Field label="Модель"><input className="form-input clean-input" value={model} onChange={(e) => setModel(e.target.value)} placeholder={defaultModels[provider]} /></Field>
          </div>
          <Field label="API ключ"><input className="form-input clean-input" type="password" value={key} onChange={(e) => setKey(e.target.value)} placeholder="Новый ключ" /></Field>
          <div className="settings-actions"><button className="btn btn-primary" onClick={saveLLM}>Сохранить</button><button className="btn btn-ghost" onClick={() => { setEditLLM(false); setKey(''); load(); }}>Отмена</button></div>
        </div>}
      </section>


      <section className="settings-edit-card reveal visible">
        <div className="settings-edit-head">
          <div>
            <h2>Funpay</h2>
            <div className="settings-readonly-grid">
              <div><span>Proxy</span><strong>{funpayProxyReady ? (data?.funpay_proxy || 'env') : 'не задан'}</strong></div>
              <div><span>Источник</span><strong>{data?.funpay_proxy ? 'настройки' : funpayProxyReady ? 'env PROXY' : '—'}</strong></div>
              <div><span>Назначение</span><strong>только парсинг</strong></div>
            </div>
          </div>
          <button className="btn btn-secondary btn-sm" onClick={() => setEditFunpay((v) => !v)}>{editFunpay ? 'Закрыть' : 'Изменить'}</button>
        </div>
        {editFunpay && <div className="settings-edit-body">
          <Field label="Funpay proxy"><input className="form-input clean-input" value={funpayProxy} onChange={(e) => setFunpayProxy(e.target.value)} placeholder="socks5://127.0.0.1:10808" /></Field>
          <div className="settings-actions"><button className="btn btn-primary" onClick={saveFunpay}>Сохранить</button><button className="btn btn-ghost" onClick={() => { setEditFunpay(false); load(); }}>Отмена</button></div>
        </div>}
      </section>

      <section className="settings-edit-card reveal visible">
        <div className="settings-edit-head">
          <div>
            <h2>Telegram</h2>
            <div className="settings-readonly-grid telegram">
              <div><span>Bot</span><strong>{data?.telegram_bot_username ? `@${data.telegram_bot_username}` : data?.telegram_has_token ? 'token задан' : 'не задан'}</strong></div>
              <div><span>Chat ID</span><strong>{data?.telegram_chat_id || 'не задан'}</strong></div>
              <div><span>Proxy</span><strong>{proxyReady ? (data?.telegram_proxy || 'env') : 'не задан'}</strong></div>
              <div><span>Уведомления</span><strong>{telegramReady ? 'включены' : 'выключены'}</strong></div>
            </div>
          </div>
          <button className="btn btn-secondary btn-sm" onClick={() => setEditTelegram((v) => !v)}>{editTelegram ? 'Закрыть' : 'Изменить'}</button>
        </div>
        {editTelegram && <div className="settings-edit-body">
          <div className="settings-form-grid two">
            <Field label="Bot token"><input className="form-input clean-input" type="password" value={tgToken} onChange={(e) => setTgToken(e.target.value)} placeholder="Новый token" /></Field>
            <Field label="Chat ID"><input className="form-input clean-input" value={tgChat} onChange={(e) => setTgChat(e.target.value)} placeholder="Chat ID" /></Field>
          </div>
          <Field label="Telegram proxy / VPN"><input className="form-input clean-input" value={tgProxy} onChange={(e) => setTgProxy(e.target.value)} placeholder="socks5://127.0.0.1:10808" /></Field>
          <div className="settings-actions wrap"><button className="btn btn-primary" onClick={saveTelegram}>Сохранить</button><button className="btn btn-secondary" onClick={syncTelegram}>Найти чат</button><button className="btn btn-secondary" onClick={testTelegram}>Тест</button><button className="btn btn-ghost" onClick={() => { setEditTelegram(false); setTgToken(''); load(); }}>Отмена</button></div>
        </div>}
      </section>
    </main>
  </>;
}

function App() {
  const [path, setPath] = useState(currentPath());
  const [toast, showToast] = useToast();
  useEffect(() => {
    const onPop = () => setPath(currentPath());
    window.addEventListener('popstate', onPop);
    return () => window.removeEventListener('popstate', onPop);
  }, []);
  let page = <HomePage showToast={showToast} />;
  if (path === '/saved') page = <SavedPage showToast={showToast} />;
  if (path === '/scheduler') page = <SchedulerPage showToast={showToast} />;
  if (path === '/settings') page = <SettingsPage showToast={showToast} />;
  return <><Background /><div className="app">{page}</div><Toast toast={toast} /></>;
}

createRoot(document.getElementById('root')).render(<App />);
