const SCROLLABLE_SELECTOR = [
  '.workspace',
  '.modal .modal-body',
  '.auth-manager-body',
  '.auth-profile-list',
  '.auth-profile-form',
  '.rate-policy-manager-layout',
].join(', ');

const IDLE_DELAY_MS = 800;
const MIN_THUMB_HEIGHT = 36;

function updateWorkspaceIndicator(workspace: HTMLElement) {
  const frame = workspace.closest<HTMLElement>('.workspace-frame');
  if (!frame) return;
  const indicator = frame.querySelector<HTMLElement>(':scope > .workspace-scrollbar');
  if (!indicator) return;
  const viewport = workspace.clientHeight;
  const scrollRange = workspace.scrollHeight - viewport;
  if (viewport <= 0 || scrollRange <= 0) {
    frame.classList.remove('is-scrolling');
    return;
  }
  const thumbHeight = Math.max(MIN_THUMB_HEIGHT, viewport * viewport / workspace.scrollHeight);
  const thumbRange = Math.max(0, viewport - thumbHeight);
  const thumbOffset = Math.min(thumbRange, Math.max(0, workspace.scrollTop / scrollRange * thumbRange));
  indicator.style.height = `${thumbHeight}px`;
  indicator.style.transform = `translate3d(0, ${thumbOffset}px, 0)`;
  frame.classList.add('is-scrolling');
}

// The workspace uses a composited overlay thumb so idle visibility changes do
// not repaint the large scrolling layer. Smaller dialogs can use their native
// thumb because their paint area is bounded.
export function followScrollActivity(): () => void {
  const timers = new Map<HTMLElement, number>();
  const activeClassTarget = (element: HTMLElement) =>
    element.matches('.workspace')
      ? element.closest<HTMLElement>('.workspace-frame') ?? element
      : element;

  const onScroll = (event: Event) => {
    const element = event.target;
    if (!(element instanceof HTMLElement) || !element.matches(SCROLLABLE_SELECTOR)) return;
    const classTarget = activeClassTarget(element);
    const existing = timers.get(classTarget);
    if (existing !== undefined) window.clearTimeout(existing);
    if (element.matches('.workspace')) updateWorkspaceIndicator(element);
    else classTarget.classList.add('is-scrolling');
    timers.set(classTarget, window.setTimeout(() => {
      classTarget.classList.remove('is-scrolling');
      timers.delete(classTarget);
    }, IDLE_DELAY_MS));
  };

  document.addEventListener('scroll', onScroll, { capture: true, passive: true });
  return () => {
    document.removeEventListener('scroll', onScroll, true);
    for (const [element, timer] of timers) {
      window.clearTimeout(timer);
      element.classList.remove('is-scrolling');
      const indicator = element.querySelector<HTMLElement>(':scope > .workspace-scrollbar');
      indicator?.style.removeProperty('height');
      indicator?.style.removeProperty('transform');
    }
    timers.clear();
  };
}
