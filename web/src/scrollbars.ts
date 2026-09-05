// Keep native scrolling; only reveal the thumb while its container is moving.
export function followScrollActivity(): () => void {
  const timers = new Map<HTMLElement, ReturnType<typeof setTimeout>>();
  const onScroll = (event: Event) => {
    const element = event.target;
    if (!(element instanceof HTMLElement) || !element.matches('.workspace, .nav')) return;
    clearTimeout(timers.get(element));
    element.classList.add('is-scrolling');
    timers.set(element, setTimeout(() => {
      element.classList.remove('is-scrolling');
      timers.delete(element);
    }, 800));
  };
  document.addEventListener('scroll', onScroll, { capture: true, passive: true });
  return () => {
    document.removeEventListener('scroll', onScroll, true);
    for (const [element, timer] of timers) {
      clearTimeout(timer);
      element.classList.remove('is-scrolling');
    }
    timers.clear();
  };
}
