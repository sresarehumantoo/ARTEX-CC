const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)');
const finePointer = window.matchMedia('(hover: hover) and (pointer: fine)');

const session = document.querySelector<HTMLElement>('[data-session]');
if (session && !reducedMotion.matches && 'IntersectionObserver' in window) {
  const lines = [...session.querySelectorAll<HTMLElement>('li[data-step]')];
  const replay = session.querySelector<HTMLButtonElement>('[data-session-replay]')!;
  const goal = session.querySelector<SVGElement>('.goal')!;
  const spinner = ['⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'];
  const counts: Record<string, number> = {};
  let next = 0;
  let visible = false;
  let timer = 0;
  let spin = 0;

  const count = (kind: string, n: number) => {
    counts[kind] = n;
    const el = session.querySelector(`[data-count="${kind}"]`);
    if (el) el.textContent = String(n);
  };

  const reset = () => {
    next = 0;
    for (const el of session.querySelectorAll('[data-on], [data-done], [data-claimed], [data-proven]')) {
      for (const attr of ['data-on', 'data-done', 'data-claimed', 'data-proven']) el.removeAttribute(attr);
    }
    for (const el of session.querySelectorAll<HTMLElement>('[data-count]')) count(el.dataset.count!, 0);
    replay.hidden = true;
  };

  const finish = (li: HTMLElement) => {
    li.dataset.done = '';
    window.clearInterval(spin);
    li.querySelector('.session-mark')!.textContent = '';
  };

  const step = () => {
    timer = 0;
    if (!visible) return;
    const li = lines[next];
    if (!li) {
      replay.hidden = false;
      return;
    }
    if (next > 0) finish(lines[next - 1]);
    li.dataset.on = '';
    const mark = li.querySelector('.session-mark')!;
    let frame = 0;
    spin = window.setInterval(() => (mark.textContent = spinner[frame++ % spinner.length]), 80);

    const n = li.dataset.step;
    for (const el of session.querySelectorAll<SVGElement>(`svg [data-step="${n}"]`)) {
      el.dataset.on = '';
      const kind = el.dataset.kind;
      if (kind) count(kind, (counts[kind] ?? 0) + 1);
      if (el.classList.contains('proof')) {
        el.addEventListener('transitionend', () => (el.dataset.done = ''), { once: true });
      }
    }
    for (const el of session.querySelectorAll<SVGElement>(`[data-claim-step="${n}"]`)) el.dataset.claimed = '';

    next++;
    if (next === lines.length) {
      window.setTimeout(() => {
        goal.dataset.proven = '';
        finish(li);
      }, 420);
    }
    timer = window.setTimeout(step, Number(li.dataset.wait));
  };

  session.dataset.motion = '';
  reset();
  new IntersectionObserver(
    ([entry]) => {
      visible = entry.isIntersecting;
      if (visible && !timer && next <= lines.length) timer = window.setTimeout(step, next ? 200 : 350);
    },
    { threshold: 0.2 },
  ).observe(session);

  replay.addEventListener('click', () => {
    window.clearTimeout(timer);
    window.clearInterval(spin);
    reset();
    timer = window.setTimeout(step, 250);
  });
}

const proxy = document.querySelector<HTMLElement>('[data-proxy]');
if (proxy && !reducedMotion.matches && 'IntersectionObserver' in window) {
  const svgNS = 'http://www.w3.org/2000/svg';
  const rows = proxy.querySelector<HTMLElement>('[data-proxy-rows]')!;
  const total = proxy.querySelector<HTMLElement>('[data-proxy-total]')!;
  const layouts = [...proxy.querySelectorAll<SVGSVGElement>('svg.proxy-flow')];
  const flows: [string, string, number, number][] = [
    ['GET', '/api/v2/users/1044', 200, 13],
    ['GET', '/api/v2/users/1045', 200, 12],
    ['GET', '/api/v2/admin/export', 403, 10],
    ['PUT', '/api/v2/users/1044/role', 200, 41],
    ['GET', '/api/v2/users/1046', 200, 12],
    ['DELETE', '/api/v2/sessions/7', 500, 88],
    ['GET', '/api/v2/users/me', 200, 11],
    ['POST', '/api/v2/graphql', 200, 57],
  ];
  type Leg = { wire: 'out' | 'in'; from?: number; to?: number; hold?: number; response?: boolean; capture?: boolean };
  type Packet = {
    svg: SVGSVGElement;
    dot: SVGRectElement;
    flow: (typeof flows)[number];
    legs: Leg[];
    leg: number;
    start: number;
    row?: HTMLElement;
  };
  const travel = 520;
  const spring = getComputedStyle(document.documentElement).getPropertyValue('--ease-spring').trim() || 'ease-out';
  const live: Packet[] = [];
  let captured = Number(total.textContent);
  let next = 0;
  let spawnAt = 0;
  let visible = false;
  let raf = 0;

  const tone = (status: number) => (status >= 500 ? 'bad' : status >= 400 ? 'warn' : '');
  const ease = (t: number) => (t < 0.5 ? 4 * t * t * t : 1 - (-2 * t + 2) ** 3 / 2);

  const capture = (svg: SVGSVGElement, [method, path]: (typeof flows)[number]) => {
    svg.querySelector('.tap')!.animate([{ opacity: 0.22 }, { opacity: 0 }], { duration: 420, easing: 'ease-out' });
    captured++;
    total.textContent = String(captured);
    const li = document.createElement('li');
    li.innerHTML =
      '<span class="text-neutral-600"></span><span class="text-sky-300"></span><span class="truncate text-neutral-300"></span><span class="proxy-status">…</span><span class="proxy-ms"></span>';
    const cells = li.children;
    cells[0].textContent = String(captured);
    cells[1].textContent = method;
    cells[2].textContent = path;
    rows.prepend(li);
    if (rows.children.length > 6) rows.lastElementChild!.remove();
    rows.animate([{ transform: 'translateY(-2rem)' }, { transform: 'none' }], { duration: 420, easing: spring });
    li.animate([{ backgroundColor: 'oklch(0.75 0.14 232 / 0.1)' }, { backgroundColor: 'transparent' }], {
      duration: 1400,
      easing: 'ease-out',
    });
    return li;
  };

  const spawn = (now: number) => {
    const svg = layouts.find((el) => el.getBoundingClientRect().width > 0);
    if (!svg) return;
    const flow = flows[next++ % flows.length];
    const vertical = svg.hasAttribute('data-vertical');
    const dot = document.createElementNS(svgNS, 'rect');
    dot.setAttribute('width', vertical ? '4' : '10');
    dot.setAttribute('height', vertical ? '10' : '4');
    dot.setAttribute('rx', '2');
    dot.setAttribute('class', 'packet');
    svg.querySelector('[data-packets]')!.append(dot);
    live.push({
      svg,
      dot,
      flow,
      leg: 0,
      start: now,
      legs: [
        { wire: 'out', from: 0, to: 1 },
        { wire: 'out', hold: 140, capture: true },
        { wire: 'in', from: 0, to: 1 },
        { wire: 'in', hold: flow[3] * 3 + 80 },
        { wire: 'in', from: 1, to: 0, response: true },
        { wire: 'out', from: 1, to: 0, response: true },
      ],
    });
  };

  const tick = (now: number) => {
    raf = 0;
    if (!visible) return;
    if (now >= spawnAt) {
      spawn(now);
      spawnAt = now + 1100 + Math.random() * 900;
    }
    for (let i = live.length - 1; i >= 0; i--) {
      const p = live[i];
      const leg = p.legs[p.leg];
      const t = Math.min(1, (now - p.start) / (leg.hold ?? travel));
      if (leg.hold === undefined) {
        const wire = p.svg.querySelector<SVGPathElement>(`[data-wire="${leg.wire}"]`)!;
        const at = wire.getPointAtLength(wire.getTotalLength() * (leg.from! + (leg.to! - leg.from!) * ease(t)));
        const w = Number(p.dot.getAttribute('width'));
        const h = Number(p.dot.getAttribute('height'));
        p.dot.setAttribute('x', String(at.x - w / 2));
        p.dot.setAttribute('y', String(at.y - h / 2));
        p.dot.setAttribute('class', leg.response ? `packet res ${tone(p.flow[2])}` : 'packet');
        p.dot.removeAttribute('opacity');
      } else {
        p.dot.setAttribute('opacity', '0.35');
      }
      if (t < 1) continue;
      if (leg.capture) p.row = capture(p.svg, p.flow);
      p.leg++;
      p.start = now;
      if (p.leg === 5 && p.row) {
        const status = p.row.querySelector('.proxy-status')!;
        status.textContent = String(p.flow[2]);
        status.className = `proxy-status ${tone(p.flow[2])}`;
        p.row.querySelector('.proxy-ms')!.textContent = `${p.flow[3]}ms`;
      }
      if (p.leg >= p.legs.length) {
        p.dot.remove();
        live.splice(i, 1);
      }
    }
    raf = requestAnimationFrame(tick);
  };

  new IntersectionObserver(([entry]) => {
    visible = entry.isIntersecting;
    if (!visible || raf) return;
    const now = performance.now();
    spawnAt = Math.max(spawnAt, now + 250);
    for (const p of live) p.start = now;
    raf = requestAnimationFrame(tick);
  }).observe(proxy);
}

// iOS Safari only applies :active styles once the page has a touchstart listener.
document.addEventListener('touchstart', () => {}, { passive: true });

if (finePointer.matches && !reducedMotion.matches) {
  let target: HTMLElement | null = null;
  let x = 0;
  let y = 0;
  let frame = 0;
  const apply = () => {
    frame = 0;
    if (!target) return;
    const rect = target.getBoundingClientRect();
    target.style.setProperty('--spot-x', `${x - rect.left}px`);
    target.style.setProperty('--spot-y', `${y - rect.top}px`);
  };
  document.addEventListener(
    'pointermove',
    (event) => {
      if (event.pointerType !== 'mouse') return;
      target = (event.target as Element).closest<HTMLElement>('[data-spotlight]');
      if (!target) return;
      x = event.clientX;
      y = event.clientY;
      frame ||= requestAnimationFrame(apply);
    },
    { passive: true },
  );
}
