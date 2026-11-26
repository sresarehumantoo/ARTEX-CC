const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)');

function scrollToTop() {
  window.scrollTo({ top: 0, behavior: reducedMotion.matches ? 'auto' : 'smooth' });
}

document.addEventListener('click', (event) => {
  if (event.defaultPrevented || event.button !== 0) return;
  if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
  const link = (event.target as Element).closest<HTMLAnchorElement>('a[href]');
  if (!link || (link.target && link.target !== '_self') || link.hasAttribute('download')) return;
  const url = new URL(link.href, location.href);
  if (url.origin !== location.origin || url.pathname !== location.pathname) return;
  if (url.search !== location.search || url.hash) return;
  event.preventDefault();
  if (location.hash) history.pushState(history.state, '', url);
  scrollToTop();
});

const nav = document.querySelector<HTMLElement>('[data-nav]');
if (nav) {
  const root = document.documentElement;
  new ResizeObserver(() => root.style.setProperty('--nav-height', `${nav.offsetHeight}px`)).observe(nav);
  const update = () => nav.toggleAttribute('data-scrolled', window.scrollY > 8);
  update();
  window.addEventListener('scroll', update, { passive: true });
  requestAnimationFrame(() => requestAnimationFrame(() => nav.setAttribute('data-nav-ready', '')));
}

const backToTop = document.querySelector<HTMLButtonElement>('[data-back-to-top]');
if (backToTop) {
  // Starlight's main pane is an isolated stacking context that sits under the fixed TOC column.
  document.body.append(backToTop);
  const update = () => {
    const long = document.documentElement.scrollHeight > window.innerHeight * 2.5;
    backToTop.hidden = !long;
    backToTop.toggleAttribute('data-visible', long && window.scrollY > window.innerHeight);
  };
  update();
  window.addEventListener('scroll', update, { passive: true });
  window.addEventListener('resize', update, { passive: true });
  backToTop.addEventListener('click', () => {
    scrollToTop();
    const heading = document.querySelector<HTMLElement>('main h1');
    if (heading) {
      heading.tabIndex = -1;
      heading.focus({ preventScroll: true });
    }
  });
}

function revealUnits(): HTMLElement[] {
  const units = new Set<HTMLElement>(document.querySelectorAll<HTMLElement>('[data-reveal]'));
  for (const group of document.querySelectorAll<HTMLElement>('[data-reveal-children]')) {
    for (const child of group.children) {
      if (child instanceof HTMLElement && !child.hasAttribute('data-reveal-children')) units.add(child);
    }
  }
  return [...units];
}

if (!reducedMotion.matches && 'IntersectionObserver' in window) {
  const pending = revealUnits().filter((el) => el.getBoundingClientRect().top > window.innerHeight);
  const observer = new IntersectionObserver(
    (entries) => {
      const visible = entries
        .filter((e) => e.isIntersecting)
        .map((e) => e.target as HTMLElement)
        .sort((a, b) => (a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING ? -1 : 1));
      visible.forEach((el, i) => {
        observer.unobserve(el);
        el.style.setProperty('--reveal-delay', `${Math.min(i, 6) * 70}ms`);
        el.dataset.revealState = 'shown';
        const done = (e: TransitionEvent) => {
          if (e.target !== el || e.propertyName !== 'opacity') return;
          el.removeEventListener('transitionend', done);
          el.style.removeProperty('--reveal-delay');
          el.dataset.revealState = 'done';
        };
        el.addEventListener('transitionend', done);
      });
    },
    { rootMargin: '0px 0px -8% 0px' },
  );
  for (const el of pending) {
    el.dataset.revealState = 'pending';
    observer.observe(el);
  }
  reducedMotion.addEventListener('change', () => {
    observer.disconnect();
    for (const el of pending) el.dataset.revealState = 'shown';
  });
}
