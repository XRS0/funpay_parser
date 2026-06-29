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
  UserCircle,
  BarChart3,
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

function getAuthToken() { return localStorage.getItem('access_token') || ''; }
function setAuthToken(t) { localStorage.setItem('access_token', t); }
function clearAuthToken() { localStorage.removeItem('access_token'); }

async function api(path, options = {}) {
  const { authRedirect = true, ...fetchOptions } = options;
  const headers = { ...(fetchOptions.headers || {}) };
  if (fetchOptions.body && !headers['Content-Type']) headers['Content-Type'] = 'application/json';
  const token = getAuthToken();
  if (token) headers.Authorization = `Bearer ${token}`;
  const resp = await fetch(path, { ...fetchOptions, headers });
  const text = await resp.text();
  let data = null;
  if (text) {
    try { data = JSON.parse(text); } catch { data = { error: text }; }
  }
  if (resp.status === 401 && authRedirect) {
    clearAuthToken();
    window.location.reload();
    return null;
  }
  if (!resp.ok) {
    throw new Error(data?.error || `HTTP ${resp.status}`);
  }
  return data;
}


function SkeletonLine({ className = '', style }) {
  return <div className={`skeleton skeleton-line ${className}`} style={style} aria-hidden='true' />;
}

function AppLoadingScreen() {
  return (
    <main className='main loading-screen' aria-busy='true' aria-label='Загрузка приложения'>
      <section className='section reveal visible loading-shell'>
        <div className='loading-brand-card card'>
          <div className='brand loading-brand'>
            <div className='brand-icon loading-brand-icon'><svg viewBox='0 0 24 24' fill='none' stroke='currentColor' strokeWidth='2.5' strokeLinecap='round' strokeLinejoin='round'><circle cx='12' cy='12' r='5' /><ellipse cx='12' cy='12' rx='9' ry='3' transform='rotate(-35 12 12)' /><circle cx='19' cy='8' r='1.5' fill='currentColor' stroke='none' /></svg></div>
            <div className='loading-brand-copy'><SkeletonLine className='skeleton-title' /><SkeletonLine style={{ width: 130 }} /></div>
          </div>
          <div className='loading-stack'>
            <SkeletonLine style={{ width: '88%' }} />
            <SkeletonLine style={{ width: '72%' }} />
            <SkeletonLine style={{ width: '46%' }} />
          </div>
        </div>
      </section>
    </main>
  );
}

function ProfileListSkeleton() {
  return (
    <div className='profiles-list compact-list skeleton-list' aria-hidden='true'>
      {Array.from({ length: 3 }).map((_, i) => (
        <div className='profile-card compact-profile-card skeleton-profile-card' key={i}>
          <div className='profile-main-line'><SkeletonLine style={{ width: 150 }} /><SkeletonLine style={{ width: 220 }} /></div>
          <div className='profile-meta-row compact'><SkeletonLine style={{ width: 54 }} /><SkeletonLine style={{ width: 78 }} /><SkeletonLine style={{ width: 62 }} /></div>
        </div>
      ))}
    </div>
  );
}

function SavedGridSkeleton({ count = 4 }) {
  return (
    <div className='saved-grid skeleton-list' aria-hidden='true'>
      {Array.from({ length: count }).map((_, i) => (
        <div className='saved-card skeleton-saved-card' key={i}>
          <div className='saved-card-main'>
            <SkeletonLine style={{ width: 190 }} />
            <SkeletonLine className='skeleton-title' style={{ width: 110 }} />
            <div className='saved-summary'><SkeletonLine style={{ width: 68 }} /><SkeletonLine style={{ width: 76 }} /><SkeletonLine style={{ width: 88 }} /></div>
          </div>
          <div className='saved-actions'><SkeletonLine style={{ width: 42, height: 42, borderRadius: 14 }} /><SkeletonLine style={{ width: 42, height: 42, borderRadius: 14 }} /></div>
        </div>
      ))}
    </div>
  );
}

function SettingsSkeleton() {
  return (
    <>
      <section className='settings-summary-row reveal visible skeleton-summary-row' aria-hidden='true'>
        {Array.from({ length: 4 }).map((_, i) => <SkeletonLine className='settings-pill skeleton-pill' key={i} />)}
      </section>
      {Array.from({ length: 3 }).map((_, i) => (
        <section className='settings-edit-card reveal visible skeleton-settings-card' key={i} aria-hidden='true'>
          <div className='settings-edit-head'>
            <div className='settings-skeleton-copy'>
              <SkeletonLine className='skeleton-title' style={{ width: 120 }} />
              <div className='settings-readonly-grid'>
                <SkeletonLine style={{ width: '100%', height: 54 }} />
                <SkeletonLine style={{ width: '100%', height: 54 }} />
                <SkeletonLine style={{ width: '100%', height: 54 }} />
              </div>
            </div>
            <SkeletonLine style={{ width: 96, height: 38, borderRadius: 999 }} />
          </div>
        </section>
      ))}
    </>
  );
}

function DetailSkeletonModal({ onClose }) {
  return <Modal title='Загрузка деталей' className='modal-wide' onClose={onClose} footer={<button className='btn btn-secondary' onClick={onClose}>Закрыть</button>}><div className='modal-loading' aria-busy='true'><SkeletonLine className='skeleton-title' style={{ width: '45%' }} /><SkeletonLine style={{ width: '90%' }} /><SkeletonLine style={{ width: '82%' }} /><div style={{ height: 16 }} />{Array.from({ length: 5 }).map((_, i) => <SkeletonLine key={i} style={{ width: `${96 - i * 8}%`, height: 36 }} />)}</div></Modal>;
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
  return ['/', '/saved', '/scheduler', '/settings', '/profile', '/admin'].includes(p) ? p : '/';
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

function NavButton({ to, children, icon, active, tour }) {
  return (
    <button type="button" data-tour={tour} className={`btn btn-secondary btn-sm nav-btn ${active ? 'active' : ''}`} aria-current={active ? 'page' : undefined} onClick={() => { if (!active) navigate(to); }}>
      {icon}
      {children}
    </button>
  );
}

function Header({ user }) {
  const path = currentPath();
  const [scrolled, setScrolled] = useState(false);
  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 10);
    onScroll();
    window.addEventListener('scroll', onScroll, { passive: true });
    return () => window.removeEventListener('scroll', onScroll);
  }, []);
  return (
    <header className={`app-header ${scrolled ? 'scrolled' : ''}`}>
      <Brand title='Funpay Parser' subtitle='тест' />
      <nav className='header-actions' aria-label='Основная навигация'>
        <NavButton to='/' tour='parser-nav' icon={<Play size={18} />} active={path === '/'}>Парсер</NavButton>
        <NavButton to='/saved' tour='saved-nav' icon={<Database size={18} />} active={path === '/saved'}>Сохранёнки</NavButton>
        <NavButton to='/scheduler' icon={<Clock size={18} />} active={path === '/scheduler'}>Расписание</NavButton>
        <NavButton to='/settings' icon={<SettingsIcon size={18} />} active={path === '/settings'}>Настройки</NavButton>
      </nav>
      <button type='button' className={`profile-chip ${path === '/profile' ? 'active' : ''}`} onClick={() => { if (path !== '/profile') navigate('/profile'); }} aria-current={path === '/profile' ? 'page' : undefined}>
        <span className='profile-chip-icon'><UserCircle size={18} /></span>
        <span className='profile-chip-name'>{user?.name || user?.email || 'Профиль'}</span>
      </button>
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

function StatusPipeline({ progress, status }) {
  const text = progress.map((p) => p.message || '').join('\n').toLowerCase();
  const statusText = String(status || '').toLowerCase();
  const steps = [
    ['fetch', 'Сбор', Search],
    ['filter', 'Фильтр', Database],
    ['preselect', 'Отбор', Plus],
    ['classify', 'LLM', SettingsIcon],
    ['result', 'Результат', Save],
  ];
  const isFinished = ['done', 'failed', 'error', 'остановлено'].some((marker) => statusText.includes(marker));
  const hasResult = text.includes('cheapest personal account') || text.includes('no personal account') || text.includes('full results saved');
  const activeIndex = isFinished || hasResult ? 4 : text.includes('classifying') || text.includes('llm') ? 3 : text.includes('limited') || text.includes('candidate') ? 2 : text.includes('filtered') ? 1 : text.includes('found') || text.includes('fetch') ? 0 : -1;
  const progressPct = Math.max(8, ((activeIndex + 1) / steps.length) * 100);
  const metrics = useMemo(() => {
    const messages = progress.map((p) => p.message || '');
    const joined = messages.join(' ');
    const llmDone = messages.filter((m) => /^\[LLM\]/.test(m.trim())).length;
    const llmChecked = messages.map((m) => m.match(/LLM checked (\d+)\/(\d+)/i)).filter(Boolean).pop();
    const candidates =
      llmChecked?.[2]
      || joined.match(/Limited LLM candidates to (\d+)/)?.[1]
      || joined.match(/Sending (\d+) filtered listings to LLM/)?.[1]
      || '0';
    const summaryClassified = joined.match(/classified[\s":]+(\d+)/i)?.[1];
    const checked = Number(llmChecked?.[1] || llmDone || 0);
    return {
      listings: Number(joined.match(/Found (\d+) listings/)?.[1] || 0),
      candidates: Number(candidates),
      classified: Number(summaryClassified || checked || 0),
      llmDone: checked,
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
  const [profilesLoaded, setProfilesLoaded] = useState(false);
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
    try {
      setProfiles(await api('/api/profiles'));
    } finally {
      setProfilesLoaded(true);
    }
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
  const showResultsBlock = hasLiveStatus && (!!status.cheapest || !!status.result_summary);

  return (
    <>
      <main className="main">
        <section className="section reveal visible search-profiles-section compact-profiles" data-tour="profiles">
          <div className="section-header profiles-header-compact">
            <div className="section-label">Профили поиска</div>
            <button className="btn btn-primary btn-sm profiles-create" onClick={() => setProfileModal({})}><Plus size={18} />Новый профиль</button>
          </div>
          {!profilesLoaded ? (
            <ProfileListSkeleton />
          ) : !profiles.length ? (
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
          <div className="card run-card" data-tour="run-card">
            <label className="run-label">Поисковый запрос</label>
            <div className="run-command">
              <div className="input-icon run-input" data-tour="query-input"><Search size={20} /><input value={query} onChange={(e) => { setQuery(e.target.value); setSelectedProfile(null); }} placeholder="например, chatgpt plus" /></div>
              {!status.running ? <button className="btn btn-primary btn-lg run-btn" data-tour="run-button" onClick={run}><Play size={20} /><span>Запустить</span></button> : <button className="btn btn-danger btn-lg stop-btn" data-tour="run-button" onClick={stop}><Square size={20} /><span>Остановить</span></button>}
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

        {showStatusBlock && <section className="section reveal visible" data-tour="status-block"><div className="section-header"><div className="section-label">Статус</div><div className={`status-badge ${status.running ? 'active' : status.status === 'Done' ? 'done' : 'idle'}`}><span className="status-dot" /><span className="status-text">{status.status || 'Ожидание'}</span></div></div><div className="card status-card"><StatusPipeline progress={safeList(status.progress)} status={status.status} /><div className="progress-terminal"><div>{safeList(status.progress).map((p, i) => <div key={`${p.time}-${i}`} className="progress-line"><span className="time">{p.time}</span> <span>{p.message}</span></div>)}</div>{status.running && <div className="progress-cursor"><span className="cursor" /></div>}</div></div></section>}
        {showResultsBlock && <ResultsView status={status} selectedProfileId={selectedProfile?.id || status.profile_id} showToast={showToast} />}
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
  const [loaded, setLoaded] = useState(false);
  const [detail, setDetail] = useState(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const load = useCallback(async () => {
    try {
      const [p, s] = await Promise.all([api('/api/profiles'), api('/api/saved_results')]);
      setProfiles(p);
      setSaved(s);
    } finally {
      setLoaded(true);
    }
  }, []);
  useEffect(() => { load().catch((err) => showToast(err.message, true)); }, [load, showToast]);
  const profileName = (id) => profiles.find((p) => p.id === id)?.name || `Профиль #${id}`;
  const del = async (id) => { if (!window.confirm('Удалить результат?')) return; try { await api(`/api/saved_results/${id}`, { method: 'DELETE' }); await load(); showToast('Результат удалён'); } catch (err) { showToast(err.message, true); } };
  const open = async (id) => {
    setDetailLoading(true);
    try { setDetail(await api(`/api/saved_results/${id}`)); }
    catch (err) { showToast(err.message, true); }
    finally { setDetailLoading(false); }
  };
  return <><main className="main"><section className="section reveal visible"><div className="section-header"><div className="section-label">История запусков</div><button className="btn btn-ghost btn-sm" disabled={!loaded} onClick={() => load().catch((err) => showToast(err.message, true))}>Обновить</button></div>{!loaded ? <SavedGridSkeleton /> : !saved.length ? <div className="empty-state"><div className="empty-title">Нет сохранённых результатов</div><div className="empty-text">Запусти парсер с профилем на главной странице, и результат появится здесь.</div></div> : <div className="saved-grid stagger visible">{saved.map((r, i) => <div className="saved-card stagger-item" key={r.id} style={{ animationDelay: `${i * 0.05}s` }} onClick={() => open(r.id)}><div className="saved-card-main"><div className="saved-date"><Clock size={18} /><span>{formatDate(r.run_at)}</span></div><div className="saved-profile"><Badge className="plan">{profileName(r.profile_id)}</Badge></div><div className="saved-price">{priceText(r.cheapest)}</div><div className="saved-summary"><Badge>{r.summary?.total_plus || 0} Plus</Badge><Badge>{r.summary?.classified || 0} LLM</Badge><Badge className="personal">{r.summary?.personal || 0} личных</Badge><Badge className="shared">{r.summary?.shared || 0} общих</Badge></div></div><div className="saved-actions"><button className="btn btn-icon" onClick={(e) => { e.stopPropagation(); open(r.id); }}><Edit3 size={18} /></button><button className="btn btn-icon" onClick={(e) => { e.stopPropagation(); del(r.id); }}><Trash2 size={18} /></button></div></div>)}</div>}</section></main>{detailLoading && !detail && <DetailSkeletonModal onClose={() => setDetailLoading(false)} />}{detail && <SavedDetail data={detail} onClose={() => setDetail(null)} onDelete={async () => { await del(detail.id); setDetail(null); }} />}</>;
}

function SavedDetail({ data, onClose, onDelete }) {
  const cheapest = data.cheapest || {};
  const allPlus = safeList(data.all_results).filter((r) => r.is_plus).sort((a, b) => (a.price || 0) - (b.price || 0)).slice(0, 50);
  return <Modal title="Детали сохранённого результата" className="modal-wide" onClose={onClose} footer={<><button className="btn btn-danger" onClick={onDelete}>Удалить</button><button className="btn btn-secondary" onClick={onClose}>Закрыть</button></>}><div className="cheapest-card card" style={{ marginBottom: 20 }}><div className="cheapest-hero"><div className="cheapest-header"><span className="cheapest-badge">Самый дешёвый личный</span><Badge className={cheapest.account_type || 'neutral'}>{typeLabel(cheapest.account_type)}</Badge></div><div className="cheapest-price">{cheapest.price ?? '—'}<span className="cheapest-currency">{cheapest.currency || ''}</span></div><div className="cheapest-title">{cheapest.title || '—'}</div></div></div><div className="table-wrap"><table><thead><tr><th>Цена</th><th>Тип</th><th>Уверенность</th><th>Заголовок</th><th>Продавец</th><th>Ссылка</th></tr></thead><tbody>{allPlus.map((r, i) => <tr key={`${r.url || i}`} className={r.account_type || ''}><td>{priceText(r)}</td><td><Badge className={r.account_type || 'neutral'}>{typeLabel(r.account_type)}</Badge></td><td>{r.confidence?.toFixed?.(2) || '—'}</td><td>{r.title || ''}</td><td>{r.seller || ''}</td><td><a href={r.url || '#'} target="_blank" rel="noreferrer">Funpay ↗</a></td></tr>)}</tbody></table></div></Modal>;
}

function SchedulerPage({ showToast }) {
  const [profiles, setProfiles] = useState([]);
  const [schedules, setSchedules] = useState([]);
  const [loaded, setLoaded] = useState(false);
  const [profileID, setProfileID] = useState('');
  const [interval, setIntervalValue] = useState(60);
  const load = useCallback(async () => {
    try {
      const [p, s] = await Promise.all([api('/api/profiles'), api('/api/schedules')]);
      setProfiles(p);
      setSchedules(s);
      setProfileID((old) => old || String(p[0]?.id || ''));
    } finally {
      setLoaded(true);
    }
  }, []);
  useEffect(() => { load().catch((err) => showToast(err.message, true)); }, [load, showToast]);
  const add = async () => { try { await api('/api/schedules', { method: 'POST', body: JSON.stringify({ profile_id: Number(profileID), interval_minutes: Number(interval), enabled: true }) }); await load(); showToast('Расписание добавлено'); } catch (err) { showToast(err.message, true); } };
  const toggle = async (id, enabled) => { try { await api(`/api/schedules/${id}`, { method: 'PUT', body: JSON.stringify({ enabled }) }); await load(); showToast(enabled ? 'Расписание активировано' : 'Расписание остановлено'); } catch (err) { showToast(err.message, true); } };
  const runNow = async (id) => { try { await api(`/api/schedules/${id}/run`, { method: 'POST' }); showToast('Запуск по расписанию начат'); } catch (err) { showToast(err.message, true); } };
  const del = async (id) => { if (!window.confirm('Удалить расписание?')) return; try { await api(`/api/schedules/${id}`, { method: 'DELETE' }); await load(); showToast('Расписание удалено'); } catch (err) { showToast(err.message, true); } };
  return <><main className="main"><section className="section reveal visible"><div className="section-header"><div className="section-label">Добавить расписание</div></div><div className="card"><div className="form-grid"><Field label="Профиль"><select className="form-input" value={profileID} disabled={!loaded || !profiles.length} onChange={(e) => setProfileID(e.target.value)}>{profiles.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}</select></Field><Field label="Интервал (минут)"><input className="form-input" type="number" value={interval} min="1" onChange={(e) => setIntervalValue(e.target.value)} /></Field></div><div style={{ marginTop: 16 }}><button className="btn btn-primary" disabled={!loaded || !profiles.length} onClick={add}>Добавить расписание</button></div></div></section><section className="section reveal visible"><div className="section-header"><div className="section-label">Активные расписания</div><button className="btn btn-ghost btn-sm" disabled={!loaded} onClick={() => load().catch((err) => showToast(err.message, true))}>Обновить</button></div>{!loaded ? <SavedGridSkeleton count={3} /> : !schedules.length ? <div className="empty-state"><div className="empty-title">Расписаний пока нет</div><div className="empty-text">Выбери профиль и интервал, чтобы парсер запускался автоматически.</div></div> : <div className="saved-grid stagger visible">{schedules.map((s, i) => <div className="saved-card stagger-item" key={s.id} style={{ animationDelay: `${i * 0.05}s` }}><div className="saved-card-main"><div className="saved-date"><Clock size={18} /><span>Интервал: {s.interval_minutes} мин</span></div><div className="saved-profile"><Badge className="plan">{s.profile_name}</Badge></div><div className="saved-summary"><Badge className={s.enabled ? 'success' : 'neutral'}>{s.enabled ? 'Активно' : 'Остановлено'}</Badge><Badge>Следующий: {formatDate(s.next_run_at, false)}</Badge><Badge>Последний: {formatDate(s.last_run_at, false)}</Badge></div></div><div className="saved-actions"><button className="btn btn-icon" onClick={() => runNow(s.id)}><Play size={18} /></button><button className="btn btn-icon" onClick={() => toggle(s.id, !s.enabled)}><Edit3 size={18} /></button><button className="btn btn-icon" onClick={() => del(s.id)}><Trash2 size={18} /></button></div></div>)}</div>}</section></main></>;
}


function TelegramLinkPanel({ account, showToast, onLinked }) {
  const [linkInfo, setLinkInfo] = useState(null);
  const [busy, setBusy] = useState(false);
  const [polling, setPolling] = useState(false);
  const lastPollError = useRef('');
  const pollTimer = useRef(null);
  const stopPolling = useCallback(() => {
    if (pollTimer.current) window.clearInterval(pollTimer.current);
    pollTimer.current = null;
    setPolling(false);
  }, []);
  useEffect(() => () => stopPolling(), [stopPolling]);
  useEffect(() => {
    if (!account?.telegram_chat_id) return;
    stopPolling();
    setLinkInfo(null);
  }, [account?.telegram_chat_id, stopPolling]);

  const confirmCode = useCallback(async (code, silent = false) => {
    if (!code) return false;
    try {
      const user = await api('/api/auth/telegram/confirm-code', { method: 'POST', body: JSON.stringify({ code }), authRedirect: false });
      lastPollError.current = '';
      stopPolling();
      setLinkInfo(null);
      onLinked?.(user);
      showToast('Telegram привязан');
      return true;
    } catch (err) {
      lastPollError.current = err.message;
      if (!silent) showToast(err.message, true);
      return false;
    }
  }, [onLinked, showToast, stopPolling]);

  const startPolling = useCallback((code) => {
    stopPolling();
    setPolling(true);
    let attempts = 0;
    pollTimer.current = window.setInterval(async () => {
      attempts += 1;
      const ok = await confirmCode(code, true);
      if (ok) return;
      const me = await api('/api/auth/me', { authRedirect: false }).catch(() => null);
      if (me?.telegram_chat_id) {
        lastPollError.current = '';
        stopPolling();
        setLinkInfo(null);
        onLinked?.(me);
        showToast('Telegram привязан');
        return;
      }
      if (attempts >= 45) {
        stopPolling();
        showToast(lastPollError.current || 'Telegram пока не ответил. Открой Telegram ещё раз и нажми Start.', true);
      }
    }, 2000);
  }, [confirmCode, onLinked, showToast, stopPolling]);

  const openTelegram = (d) => {
    if (d?.deep_link) window.open(d.deep_link, '_blank', 'noopener,noreferrer');
  };

  const createCode = async () => {
    setBusy(true);
    try {
      const d = await api('/api/auth/telegram/link-code', { method: 'POST', authRedirect: false });
      lastPollError.current = '';
      setLinkInfo(d);
      openTelegram(d);
      startPolling(d.code);
      showToast('Открыл Telegram по deep link. Нажми Start в боте — сайт подхватит привязку сам.');
    } catch (err) { showToast(err.message, true); }
    finally { setBusy(false); }
  };

  return <div className='telegram-link-box'>
    <div>
      <div className='telegram-link-title'>{account?.telegram_chat_id ? 'Telegram привязан' : 'Привязка Telegram'}</div>
      {(account?.telegram_username || account?.telegram_chat_id) && <div className='telegram-link-text'>{account?.telegram_username ? `@${account.telegram_username}` : `Chat ID ${account.telegram_chat_id}`}</div>}
    </div>
    {linkInfo ? <div className='telegram-deeplink-panel'>
      <span>{polling ? 'Жду Start в Telegram…' : 'Можно открыть Telegram ещё раз'}</span>
      <button className='btn btn-secondary btn-sm' disabled={busy || !linkInfo.deep_link} onClick={() => { openTelegram(linkInfo); startPolling(linkInfo.code); }}>Открыть Telegram</button>
    </div> : <button className='btn btn-secondary btn-sm' disabled={busy} onClick={createCode}>{account?.telegram_chat_id ? 'Перепривязать Telegram' : 'Войти через Telegram'}</button>}
  </div>;
}

function ProfilePage({ showToast, user, onUserUpdate }) {
  const [data, setData] = useState(null);
  const [name, setName] = useState(user?.name || '');
  const [editing, setEditing] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const load = useCallback(async () => {
    try {
      const d = await api('/api/profile');
      setData(d);
      if (d?.user) {
        onUserUpdate?.(d.user);
        setName(d.user.name || '');
      }
    } finally { setLoaded(true); }
  }, [onUserUpdate]);
  useEffect(() => { load().catch((err) => showToast(err.message, true)); }, [load, showToast]);
  const saveName = async () => {
    try {
      const u = await api('/api/auth/profile', { method: 'PUT', body: JSON.stringify({ name }), authRedirect: false });
      onUserUpdate?.(u);
      setEditing(false);
      await load();
      showToast('Профиль обновлён');
    } catch (err) { showToast(err.message, true); }
  };
  if (!loaded) return <main className='main profile-page'><SettingsSkeleton /></main>;
  const u = data?.user || user || {};
  const st = data?.stats || {};
  return <main className='main profile-page'>
    <section className='profile-hero-card reveal visible'>
      <div className='profile-avatar'>{(u.name || u.email || 'U').slice(0, 1).toUpperCase()}</div>
      <div className='profile-hero-main'>
        <div className='section-label'>Профиль</div>
        {editing ? <div className='profile-name-edit'><input className='form-input clean-input' value={name} onChange={(e) => setName(e.target.value)} /><button className='btn btn-primary btn-sm' onClick={saveName}>Сохранить</button><button className='btn btn-ghost btn-sm' onClick={() => { setEditing(false); setName(u.name || ''); }}>Отмена</button></div> : <><h2>{u.name || 'Пользователь'}</h2><p>{u.email}</p></>}
      </div>
      {!editing && <button className='btn btn-secondary btn-sm' onClick={() => setEditing(true)}>Изменить имя</button>}
    </section>
    <section className='usage-grid reveal visible'>
      <div className='usage-card'><span>Профили</span><strong>{st.profiles || 0}</strong></div>
      <div className='usage-card'><span>Сохранёнки</span><strong>{st.saved_results || 0}/10</strong></div>
      <div className='usage-card'><span>Расписания</span><strong>{st.schedules || 0}</strong></div>
      <div className='usage-card'><span>Проверено LLM</span><strong>{st.total_llm || 0}</strong></div>
      <div className='usage-card'><span>Plus найдено</span><strong>{st.total_plus || 0}</strong></div>
      <div className='usage-card'><span>Лучшая цена</span><strong>{st.best_price ? `${st.best_price} ${st.best_currency || ''}` : '—'}</strong></div>
    </section>
    <section className='settings-edit-card reveal visible'>
      <div className='settings-edit-head'><div><h2>Telegram</h2><p className='muted-copy'>Уведомления отправляются только в Telegram, привязанный к твоему аккаунту.</p></div><Badge className={st.telegram_linked ? 'success' : 'neutral'}>{st.telegram_linked ? 'привязан' : 'не привязан'}</Badge></div>
      <TelegramLinkPanel account={u} showToast={showToast} onLinked={(next) => { onUserUpdate?.(next); load(); }} />
    </section>
    <section className='settings-edit-card reveal visible'>
      <div className='settings-edit-head'><div><h2>Использование</h2><div className='settings-readonly-grid'><div><span>Последний запуск</span><strong>{st.last_run_at ? formatDate(st.last_run_at) : '—'}</strong></div><div><span>Хранение</span><strong>только последние 10 запусков</strong></div><div><span>Изоляция</span><strong>данные видит только владелец</strong></div></div></div><BarChart3 size={28} /></div>
    </section>
  </main>;
}

function SettingsPage({ showToast, onLogout }) {
  const [account, setAccount] = useState(null);
  const [loaded, setLoaded] = useState(false);
  const [editAccount, setEditAccount] = useState(false);
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [accountBusy, setAccountBusy] = useState(false);
  const load = useCallback(async () => {
    try {
      const me = await api('/api/auth/me', { authRedirect: false }).catch(() => null);
      setAccount(me);
    } finally {
      setLoaded(true);
    }
  }, []);
  useEffect(() => { load().catch((err) => showToast(err.message, true)); }, [load, showToast]);
  const savePassword = async () => {
    if (newPassword.length < 6) { showToast('Новый пароль должен быть минимум 6 символов', true); return; }
    if (newPassword !== confirmPassword) { showToast('Пароли не совпадают', true); return; }
    setAccountBusy(true);
    try {
      await api('/api/auth/password', { method: 'POST', body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }), authRedirect: false });
      setCurrentPassword('');
      setNewPassword('');
      setConfirmPassword('');
      setEditAccount(false);
      showToast('Пароль изменён, войдите заново');
      window.setTimeout(onLogout, 650);
    } catch (err) { showToast(err.message, true); }
    finally { setAccountBusy(false); }
  };
  if (!loaded) {
    return <main className="main settings-page lean-settings"><SettingsSkeleton /></main>;
  }
  const linked = !!account?.telegram_chat_id;
  return <main className="main settings-page user-settings-page">
    <section className="settings-human-hero reveal visible">
      <div className="settings-human-copy">
        <span className="section-kicker">Личный кабинет</span>
      </div>
      <div className="settings-human-status">
        <Badge className={linked ? 'success' : 'neutral'}>{linked ? 'Telegram привязан' : 'Telegram не привязан'}</Badge>
      </div>
    </section>

    <section className="settings-human-grid reveal visible">
      <article className="settings-human-card account-settings-card">
        <div className="settings-human-card-head">
          <div className="settings-human-icon"><UserCircle size={22} /></div>
          <div>
            <h2>Аккаунт</h2>
            <p>Имя, вход и безопасная смена пароля.</p>
          </div>
        </div>
        <div className="settings-readable-list">
          <div><span>Имя</span><strong>{account?.name || '—'}</strong></div>
          <div><span>Email</span><strong>{account?.email || 'текущий пользователь'}</strong></div>
          <div><span>Роль</span><strong>{account?.role || 'user'}</strong></div>
        </div>
        <div className="settings-actions account-actions">
          <button className="btn btn-secondary btn-sm" onClick={() => setEditAccount((v) => !v)}>{editAccount ? 'Закрыть' : 'Поменять пароль'}</button>
          <button className="btn btn-danger btn-sm" onClick={onLogout}><ShieldCheck size={18} />Выйти</button>
        </div>
        {editAccount && <div className="settings-edit-body human-edit-body">
          <div className="settings-form-grid two">
            <Field label="Текущий пароль"><input className="form-input clean-input" type="password" value={currentPassword} onChange={(e) => setCurrentPassword(e.target.value)} placeholder="Текущий пароль" /></Field>
            <Field label="Новый пароль"><input className="form-input clean-input" type="password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} placeholder="Минимум 6 символов" /></Field>
          </div>
          <Field label="Повтор нового пароля"><input className="form-input clean-input" type="password" value={confirmPassword} onChange={(e) => setConfirmPassword(e.target.value)} placeholder="Повтори новый пароль" /></Field>
          <div className="settings-actions"><button className={`btn btn-primary ${accountBusy ? 'btn-loading' : ''}`} disabled={accountBusy} onClick={savePassword}>Сменить пароль</button><button className="btn btn-ghost" onClick={() => { setEditAccount(false); setCurrentPassword(''); setNewPassword(''); setConfirmPassword(''); }}>Отмена</button></div>
        </div>}
      </article>

      <article className="settings-human-card telegram-account-card">
        <div className="settings-human-card-head">
          <div className="settings-human-icon"><Bot size={22} /></div>
          <div>
            <h2>Telegram</h2>
            <p>Привязка нужна только для твоих персональных отчётов.</p>
          </div>
        </div>
        <TelegramLinkPanel account={account} showToast={showToast} onLinked={(u) => { setAccount(u); load(); }} />
      </article>

      <article className="settings-human-card usage-note-card">
        <div className="settings-human-card-head">
          <div className="settings-human-icon"><Database size={22} /></div>
          <div>
            <h2>Данные</h2>
            <p>История и результаты разделены между пользователями.</p>
          </div>
        </div>
        <div className="settings-readable-list compact">
          <div><span>Хранение</span><strong>последние 10 запусков</strong></div>
          <div><span>Доступ</span><strong>только владелец</strong></div>
          <div><span>Отчёты</span><strong>{linked ? 'в Telegram' : 'после привязки'}</strong></div>
        </div>
      </article>
    </section>
  </main>;
}

function AdminSettingsPage({ showToast }) {
  const [code, setCode] = useState(sessionStorage.getItem('admin_code') || '');
  const [unlocked, setUnlocked] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [saving, setSaving] = useState(false);
  const [provider, setProvider] = useState('fireworks');
  const [model, setModel] = useState('');
  const [llmKey, setLlmKey] = useState('');
  const [telegramToken, setTelegramToken] = useState('');
  const [telegramProxy, setTelegramProxy] = useState('');
  const [telegramChat, setTelegramChat] = useState('');
  const [funpayProxy, setFunpayProxy] = useState('');
  const [meta, setMeta] = useState(null);
  const adminHeaders = useCallback(() => ({ 'X-Admin-Code': code.trim() }), [code]);
  const hydrate = useCallback((d) => {
    setMeta(d);
    setProvider(d.llm_provider || 'fireworks');
    setModel(d.llm_model || '');
    setLlmKey('');
    setTelegramToken('');
    setTelegramProxy(d.telegram_proxy || '');
    setTelegramChat(d.telegram_chat_id || '');
    setFunpayProxy(d.funpay_proxy || '');
  }, []);
  const unlock = async () => {
    try {
      const d = await api('/api/admin/settings', { headers: adminHeaders(), authRedirect: false });
      sessionStorage.setItem('admin_code', code.trim());
      hydrate(d);
      setUnlocked(true);
      setLoaded(true);
      showToast('Админка открыта');
    } catch (err) { showToast(err.message || 'Неверный код', true); }
  };
  const save = async () => {
    setSaving(true);
    try {
      const body = {
        llm_provider: provider,
        llm_model: model.trim(),
        telegram_chat_id: telegramChat.trim(),
        telegram_proxy: telegramProxy.trim(),
        funpay_proxy: funpayProxy.trim(),
      };
      if (llmKey.trim()) body.llm_api_key = llmKey.trim();
      if (telegramToken.trim()) body.telegram_bot_token = telegramToken.trim();
      const d = await api('/api/admin/settings', { method: 'PUT', headers: adminHeaders(), body: JSON.stringify(body), authRedirect: false });
      hydrate(d);
      showToast('Системные настройки сохранены');
    } catch (err) { showToast(err.message, true); }
    finally { setSaving(false); }
  };
  useEffect(() => {
    if (!code.trim()) return;
    api('/api/admin/settings', { headers: adminHeaders(), authRedirect: false }).then((d) => { hydrate(d); setUnlocked(true); setLoaded(true); }).catch(() => setLoaded(true));
  }, []);
  if (!unlocked) {
    return <main className="main admin-page">
      <section className="admin-gate-card reveal visible">
        <div className="settings-human-icon"><ShieldCheck size={24} /></div>
        <div>
          <span className="section-kicker">Админка</span>
          <h2>Системные настройки</h2>
          <p>Введите код администратора, чтобы открыть LLM, Telegram bot и прокси. Для старта код: 1082.</p>
        </div>
        <Field label="Код администратора"><input className="form-input clean-input" type="password" value={code} onChange={(e) => setCode(e.target.value)} placeholder="1082" onKeyDown={(e) => { if (e.key === 'Enter') unlock(); }} /></Field>
        <button className="btn btn-primary" onClick={unlock}>Войти в админку</button>
      </section>
    </main>;
  }
  if (!loaded) return <main className="main admin-page"><SettingsSkeleton /></main>;
  return <main className="main admin-page">
    <section className="settings-human-hero reveal visible admin-hero">
      <div className="settings-human-copy">
        <span className="section-kicker">Админка</span>
        <h2>Системная конфигурация</h2>
        <p>Эти параметры влияют на всех пользователей и не показываются в обычных настройках.</p>
      </div>
      <button className="btn btn-ghost btn-sm" onClick={() => { sessionStorage.removeItem('admin_code'); setUnlocked(false); setCode(''); }}>Закрыть доступ</button>
    </section>
    <section className="admin-settings-grid reveal visible">
      <article className="settings-human-card">
        <div className="settings-human-card-head"><div className="settings-human-icon"><KeyRound size={22} /></div><div><h2>LLM</h2><p>Провайдер, модель и API ключ классификации.</p></div></div>
        <div className="settings-readable-list compact"><div><span>Статус</span><strong>{meta?.has_key ? 'ключ задан' : 'ключ не задан'}</strong></div><div><span>Текущий ключ</span><strong>{meta?.llm_api_key || '—'}</strong></div></div>
        <div className="settings-form-grid two">
          <Field label="Провайдер"><select className="form-input clean-input" value={provider} onChange={(e) => setProvider(e.target.value)}><option value="fireworks">Fireworks</option><option value="openrouter">OpenRouter</option></select></Field>
          <Field label="Модель"><input className="form-input clean-input" value={model} onChange={(e) => setModel(e.target.value)} placeholder={defaultModels[provider]} /></Field>
        </div>
        <Field label="API ключ"><input className="form-input clean-input" type="password" value={llmKey} onChange={(e) => setLlmKey(e.target.value)} placeholder="Новый ключ, пусто = без изменений" /></Field>
      </article>
      <article className="settings-human-card">
        <div className="settings-human-card-head"><div className="settings-human-icon"><Bot size={22} /></div><div><h2>Telegram bot</h2><p>Бот, общий chat fallback и отдельный proxy/VPN для Telegram.</p></div></div>
        <div className="settings-readable-list compact"><div><span>Bot</span><strong>{meta?.telegram_bot_username ? `@${meta.telegram_bot_username}` : meta?.telegram_has_token ? 'token задан' : 'не задан'}</strong></div><div><span>Proxy</span><strong>{meta?.telegram_proxy_active ? 'активен' : 'не задан'}</strong></div></div>
        <div className="settings-form-grid two">
          <Field label="Bot token"><input className="form-input clean-input" type="password" value={telegramToken} onChange={(e) => setTelegramToken(e.target.value)} placeholder="Новый token, пусто = без изменений" /></Field>
          <Field label="Fallback Chat ID"><input className="form-input clean-input" value={telegramChat} onChange={(e) => setTelegramChat(e.target.value)} placeholder="не обязателен" /></Field>
        </div>
        <Field label="Telegram proxy / VPN"><input className="form-input clean-input" value={telegramProxy} onChange={(e) => setTelegramProxy(e.target.value)} placeholder="socks5://127.0.0.1:10808" /></Field>
      </article>
      <article className="settings-human-card">
        <div className="settings-human-card-head"><div className="settings-human-icon"><Route size={22} /></div><div><h2>Funpay proxy</h2><p>Отдельный proxy только для парсинга Funpay.</p></div></div>
        <div className="settings-readable-list compact"><div><span>Статус</span><strong>{meta?.funpay_proxy_active ? 'активен' : 'не задан'}</strong></div><div><span>Текущее значение</span><strong>{meta?.funpay_proxy || 'env / пусто'}</strong></div></div>
        <Field label="Funpay proxy"><input className="form-input clean-input" value={funpayProxy} onChange={(e) => setFunpayProxy(e.target.value)} placeholder="socks5://127.0.0.1:10808" /></Field>
      </article>
    </section>
    <div className="admin-save-bar"><button className={`btn btn-primary ${saving ? 'btn-loading' : ''}`} disabled={saving} onClick={save}><Save size={18} />Сохранить системные настройки</button></div>
  </main>;
}




function GuidedOnboarding({ user, onComplete, showToast }) {
  const [step, setStep] = useState(0);
  const [saving, setSaving] = useState(false);
  const [targetRect, setTargetRect] = useState(null);
  const steps = [
    { selector: '[data-tour="parser-nav"]', title: 'Главная страница парсинга', text: 'Здесь запускаются проверки Funpay. Эта вкладка всегда возвращает тебя к рабочему экрану.', placement: 'bottom' },
    { selector: '[data-tour="profiles"]', title: 'Профили поиска', text: 'Сохраняй частые запросы отдельными профилями: категория, лимиты, Deep mode и количество кандидатов.', placement: 'right' },
    { selector: '[data-tour="query-input"]', title: 'Запрос и параметры', text: 'Введи, что искать на Funpay. Ниже можно поменять категорию, кандидатов и глубину проверки.', placement: 'bottom' },
    { selector: '[data-tour="run-button"]', title: 'Кнопка запуска', text: 'Нажми “Запустить”, чтобы начать реальный сбор, фильтрацию и проверку через LLM.', placement: 'left' },
    { selector: '[data-tour="saved-nav"]', title: 'Где смотреть результат', text: 'После завершения результат сохраняется в “Сохранёнках”: там история последних запусков и подробности.', placement: 'bottom' },
  ];
  const active = steps[step];
  const finish = useCallback(async () => {
    if (saving) return;
    setSaving(true);
    try {
      const updated = await api('/api/auth/onboarding', { method: 'POST', authRedirect: false });
      onComplete(updated || { ...user, onboardingCompleted: true });
      showToast?.('Onboarding завершён');
    } catch (err) {
      showToast?.(err.message || 'Не удалось сохранить onboarding', true);
    } finally {
      setSaving(false);
    }
  }, [onComplete, saving, showToast, user]);

  useEffect(() => {
    const update = () => {
      const el = document.querySelector(active.selector);
      if (!el) {
        setTargetRect(null);
        return;
      }
      const r = el.getBoundingClientRect();
      setTargetRect({ top: r.top, left: r.left, width: r.width, height: r.height });
      el.scrollIntoView?.({ block: 'center', inline: 'center', behavior: 'smooth' });
    };
    update();
    const id = window.setTimeout(update, 260);
    window.addEventListener('resize', update);
    window.addEventListener('scroll', update, true);
    return () => {
      window.clearTimeout(id);
      window.removeEventListener('resize', update);
      window.removeEventListener('scroll', update, true);
    };
  }, [active.selector]);

  useEffect(() => {
    const onKey = (event) => {
      if (event.key === 'Escape') finish();
      if (event.key === 'ArrowRight') setStep((v) => Math.min(steps.length - 1, v + 1));
      if (event.key === 'ArrowLeft') setStep((v) => Math.max(0, v - 1));
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [finish, steps.length]);

  const pad = 10;
  const rect = targetRect ? {
    top: Math.max(8, targetRect.top - pad),
    left: Math.max(8, targetRect.left - pad),
    width: targetRect.width + pad * 2,
    height: targetRect.height + pad * 2,
  } : null;
  const vw = window.innerWidth;
  const vh = window.innerHeight;
  const tooltipWidth = Math.min(390, vw - 28);
  const baseTop = rect ? rect.top + rect.height + 16 : 150;
  const sideTop = rect ? rect.top + Math.min(10, rect.height / 2) : 150;
  const tooltip = (() => {
    if (!rect) return { top: 150, left: Math.max(14, (vw - tooltipWidth) / 2) };
    if (active.placement === 'left') return { top: Math.min(vh - 260, Math.max(14, sideTop)), left: Math.max(14, rect.left - tooltipWidth - 18) };
    if (active.placement === 'right') return { top: Math.min(vh - 260, Math.max(14, sideTop)), left: Math.min(vw - tooltipWidth - 14, rect.left + rect.width + 18) };
    return { top: Math.min(vh - 260, Math.max(14, baseTop)), left: Math.min(vw - tooltipWidth - 14, Math.max(14, rect.left + rect.width / 2 - tooltipWidth / 2)) };
  })();

  return <div className="tour-layer" aria-live="polite">
    <div className="tour-scrim" />
    {rect && <div className="tour-spotlight" style={{ top: rect.top, left: rect.left, width: rect.width, height: rect.height }} />}
    <div className="tour-popover" style={{ top: tooltip.top, left: tooltip.left, width: tooltipWidth }} role="dialog" aria-label="Подсказка onboarding">
      <div className="tour-kicker">Шаг {step + 1} из {steps.length}</div>
      <h2>{active.title}</h2>
      <p>{active.text}</p>
      <div className="tour-progress" aria-hidden="true">{steps.map((_, i) => <span key={i} className={i <= step ? 'active' : ''} />)}</div>
      <div className="tour-actions">
        <button type="button" className="btn btn-ghost btn-sm" onClick={finish} disabled={saving}>Пропустить</button>
        {step > 0 && <button type="button" className="btn btn-secondary btn-sm" onClick={() => setStep((v) => Math.max(0, v - 1))} disabled={saving}>Назад</button>}
        <button type="button" className={`btn btn-primary btn-sm ${saving ? 'btn-loading' : ''}`} onClick={step >= steps.length - 1 ? finish : () => setStep((v) => v + 1)} disabled={saving}>{step >= steps.length - 1 ? 'Начать' : 'Далее'}</button>
      </div>
    </div>
  </div>;
}

function TelegramRequiredPage({ user, showToast, onLinked }) {
  return <main className="main telegram-required-page">
    <section className="section reveal visible telegram-required-section">
      <div className="telegram-required-card card">
        <div className="telegram-required-icon"><Bot size={30} /></div>
        <div className="section-label">Обязательная регистрация</div>
        <h1>Привяжи Telegram</h1>
        <TelegramLinkPanel account={user} showToast={showToast} onLinked={onLinked} />
      </div>
    </section>
  </main>;
}


function LoginPage({ onLogin, showToast }) {
  const [isRegister, setIsRegister] = useState(false);
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [telegramEnabled, setTelegramEnabled] = useState(false);

  useEffect(() => {
    api('/api/auth/config', { authRedirect: false })
      .then((d) => setTelegramEnabled(!!d?.telegram_login_enabled))
      .catch(() => setTelegramEnabled(false));
  }, []);

  const submit = async (e) => {
    e.preventDefault();
    setLoading(true);
    try {
      const endpoint = isRegister ? '/api/auth/register' : '/api/auth/login';
      const data = await api(endpoint, { method: 'POST', body: JSON.stringify(isRegister ? { name, email, password } : { email, password }), authRedirect: false });
      setAuthToken(data.access_token);
      onLogin();
    } catch (err) {
      showToast(err.message, true);
    } finally {
      setLoading(false);
    }
  };

  const telegramLogin = async () => {
    if (!telegramEnabled) {
      showToast('Telegram-вход пока не настроен', true);
      return;
    }
    setLoading(true);
    try {
      const d = await api('/api/auth/telegram/login-code', { method: 'POST', authRedirect: false });
      if (d.deep_link) window.open(d.deep_link, '_blank', 'noopener,noreferrer');
      showToast('Открыл Telegram. Нажми Start в боте — вход завершится автоматически.');
      let attempts = 0;
      const timer = window.setInterval(async () => {
        attempts += 1;
        try {
          const data = await api('/api/auth/telegram/confirm-login-code', { method: 'POST', body: JSON.stringify({ code: d.code }), authRedirect: false });
          window.clearInterval(timer);
          setAuthToken(data.access_token);
          setLoading(false);
          onLogin();
        } catch {
          if (attempts >= 45) {
            window.clearInterval(timer);
            setLoading(false);
            showToast('Не дождался Start в Telegram. Попробуй ещё раз.', true);
          }
        }
      }, 2000);
    } catch (err) {
      setLoading(false);
      showToast(err.message, true);
    }
  };

  return (
    <main className='main' style={{ maxWidth: 420, margin: '0 auto', paddingTop: 80 }}>
      <section className='section reveal visible'>
        <div className='card' style={{ padding: 32 }}>
          <div className='brand' style={{ justifyContent: 'center', marginBottom: 24 }}>
            <div className='brand-icon'><svg viewBox='0 0 24 24' fill='none' stroke='currentColor' strokeWidth='2.5' strokeLinecap='round' strokeLinejoin='round'><circle cx='12' cy='12' r='5' /><ellipse cx='12' cy='12' rx='9' ry='3' transform='rotate(-35 12 12)' /><circle cx='19' cy='8' r='1.5' fill='currentColor' stroke='none' /></svg></div>
            <div><h1>Funpay Parser</h1></div>
          </div>
          <form onSubmit={submit}>
            {isRegister && <Field label='Имя'><input type='text' value={name} onChange={(e) => setName(e.target.value)} required placeholder='Как тебя показывать в профиле' /></Field>}
            <Field label='Email'><input type='email' value={email} onChange={(e) => setEmail(e.target.value)} required /></Field>
            <Field label='Пароль'><input type='password' value={password} onChange={(e) => setPassword(e.target.value)} required minLength={6} /></Field>
            <button type='submit' className='btn btn-primary btn-lg' disabled={loading} style={{ width: '100%', marginTop: 8 }}>{isRegister ? 'Создать аккаунт' : 'Войти'}</button>
          </form>
          <div style={{ textAlign: 'center', marginTop: 16 }}>
            <button type='button' className='btn btn-ghost btn-sm' onClick={() => setIsRegister((v) => !v)}>{isRegister ? 'Уже есть аккаунт? Войти' : 'Нет аккаунта? Создать'}</button>
          </div>
          <div style={{ textAlign: 'center', marginTop: 20, borderTop: '1px solid rgba(255,255,255,0.08)', paddingTop: 20 }}>
            <button type='button' className='btn btn-secondary btn-sm' onClick={telegramLogin} disabled={loading || !telegramEnabled}><Bot size={18} />Войти через Telegram</button>
          </div>
        </div>
      </section>
    </main>
  );
}

function App() {
  const [path, setPath] = useState(currentPath());
  const [toast, showToast] = useToast();
  const [authenticated, setAuthenticated] = useState(!!getAuthToken());
  const [currentUser, setCurrentUser] = useState(null);
  const [checking, setChecking] = useState(true);

  useEffect(() => {
    const onPop = () => setPath(currentPath());
    window.addEventListener('popstate', onPop);
    return () => window.removeEventListener('popstate', onPop);
  }, []);

  useEffect(() => {
    let mounted = true;
    async function init() {
      try {
        const cfg = await api('/api/auth/config', { authRedirect: false });
        if (!cfg?.auth_enabled) {
          if (mounted) {
            setAuthenticated(true);
            setChecking(false);
          }
          return;
        }
      } catch {
        // Старые/неполные сборки без auth-config считаем защищёнными, чтобы не открыть API случайно.
      }
      if (getAuthToken()) {
        try { const me = await api('/api/auth/me', { authRedirect: false }); if (mounted) { setCurrentUser(me); setAuthenticated(true); } }
        catch { clearAuthToken(); if (mounted) setAuthenticated(false); }
        finally { if (mounted) setChecking(false); }
        return;
      }
      try {
        const data = await api('/api/auth/refresh', { method: 'POST', authRedirect: false });
        setAuthToken(data.access_token);
        if (mounted) { setCurrentUser(data.user || null); setAuthenticated(true); }
      } catch {
        if (mounted) setAuthenticated(false);
      } finally {
        if (mounted) setChecking(false);
      }
    }
    init();
    return () => { mounted = false; };
  }, []);

  const logout = async () => {
    try { await api('/api/auth/logout', { method: 'POST', authRedirect: false }); } catch {}
    clearAuthToken();
    window.location.reload();
  };

  if (checking) {
    return <><Background /><div className='app'><AppLoadingScreen /></div><Toast toast={toast} /></>;
  }
  if (!authenticated) {
    return <><Background /><div className='app'><LoginPage onLogin={async () => { const me = await api('/api/auth/me', { authRedirect: false }).catch(() => null); setCurrentUser(me); setAuthenticated(true); }} showToast={showToast} /></div><Toast toast={toast} /></>;
  }
  let page = <HomePage showToast={showToast} />;
  if (path === '/saved') page = <SavedPage showToast={showToast} />;
  if (path === '/scheduler') page = <SchedulerPage showToast={showToast} />;
  if (path === '/settings') page = <SettingsPage showToast={showToast} onLogout={logout} />;
  if (path === '/profile') page = <ProfilePage showToast={showToast} user={currentUser} onUserUpdate={setCurrentUser} />;
  if (path === '/admin') page = <AdminSettingsPage showToast={showToast} />;
  const needsTelegram = currentUser && !currentUser.telegram_chat_id;
  if (needsTelegram) {
    return <><Background /><div className='app'><Header user={currentUser} /><TelegramRequiredPage user={currentUser} showToast={showToast} onLinked={(u) => setCurrentUser(u)} /></div><Toast toast={toast} /></>;
  }
  const showOnboarding = currentUser && currentUser.onboardingCompleted !== true;
  return <><Background /><div className='app'><Header user={currentUser} />{page}</div>{showOnboarding && <GuidedOnboarding user={currentUser} showToast={showToast} onComplete={(u) => setCurrentUser(u)} />}<Toast toast={toast} /></>;
}

createRoot(document.getElementById('root')).render(<App />);
