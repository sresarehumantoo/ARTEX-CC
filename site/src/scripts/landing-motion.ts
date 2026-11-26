const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)');
const finePointer = window.matchMedia('(hover: hover) and (pointer: fine)');

const stream = document.querySelector<HTMLElement>('[data-stream]');
if (
  stream &&
  !reducedMotion.matches &&
  'IntersectionObserver' in window &&
  stream.getBoundingClientRect().top > window.innerHeight
) {
  const lines = [...stream.children] as HTMLElement[];
  lines.forEach((li, i) => li.style.setProperty('--stream-i', String(i)));
  stream.dataset.streamState = 'pending';
  const observer = new IntersectionObserver(
    ([entry]) => {
      if (!entry.isIntersecting) return;
      observer.disconnect();
      stream.dataset.streamState = 'running';
      const last = lines[lines.length - 1];
      const done = (e: AnimationEvent) => {
        if (e.target !== last) return;
        last.removeEventListener('animationend', done);
        stream.dataset.streamState = 'done';
      };
      last.addEventListener('animationend', done);
    },
    { threshold: 0.2 },
  );
  observer.observe(stream);
}

if ('IntersectionObserver' in window) {
  const offscreen = new IntersectionObserver((entries) => {
    for (const entry of entries) entry.target.toggleAttribute('data-offscreen', !entry.isIntersecting);
  });
  for (const el of [document.getElementById('hero'), stream]) if (el) offscreen.observe(el);
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
      target = (event.target as Element).closest<HTMLElement>('[data-spotlight], [data-spotlight-area]');
      if (!target) return;
      x = event.clientX;
      y = event.clientY;
      frame ||= requestAnimationFrame(apply);
    },
    { passive: true },
  );
}
