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
