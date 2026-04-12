export function reveal(node: HTMLElement, options?: { delay?: number; y?: number }) {
  const { delay = 0, y = 20 } = options ?? {};
  const prefersReduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  if (!prefersReduced) {
    node.style.cssText += `opacity:0;transform:translateY(${y}px);transition:opacity var(--duration-slow) var(--ease-expo-out) ${delay}ms, transform var(--duration-slow) var(--ease-expo-out) ${delay}ms`;
  }
  const obs = new IntersectionObserver(
    ([entry]) => {
      if (entry.isIntersecting) {
        node.style.opacity = '1';
        node.style.transform = 'translateY(0)';
        obs.disconnect();
      }
    },
    { threshold: 0.15 }
  );
  obs.observe(node);
  return { destroy: () => obs.disconnect() };
}
