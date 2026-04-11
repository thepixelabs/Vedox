/*
 * analytics.ts — Thin wrapper around Plausible-style event tracking.
 *
 * All tracking is gated on PUBLIC_PLAUSIBLE_DOMAIN. If the env var is
 * unset at build time, calls are no-ops and no script is loaded. The
 * product itself is zero-telemetry; this file exists only for the
 * marketing site, and even here it only loads when explicitly configured.
 */

import { browser } from '$app/environment';
import { env } from '$env/dynamic/public';

type Plausible = (event: string, opts?: { props?: Record<string, string> }) => void;

declare global {
	interface Window {
		plausible?: Plausible & { q?: unknown[] };
	}
}

export function initAnalytics(): void {
	if (!browser) return;
	const domain = env.PUBLIC_PLAUSIBLE_DOMAIN;
	if (!domain) return;
	if (document.querySelector('script[data-plausible]')) return;

	const src = env.PUBLIC_PLAUSIBLE_SRC || 'https://plausible.io/js/script.outbound-links.js';
	const s = document.createElement('script');
	s.defer = true;
	s.setAttribute('data-domain', domain);
	s.setAttribute('data-plausible', 'true');
	s.src = src;
	document.head.appendChild(s);

	// Stub queue so early track() calls aren't lost.
	window.plausible =
		window.plausible ||
		function (...args: unknown[]) {
			(window.plausible!.q = window.plausible!.q || []).push(args);
		};
}

export function track(event: string, props?: Record<string, string>): void {
	if (!browser) return;
	if (!env.PUBLIC_PLAUSIBLE_DOMAIN) return;
	try {
		window.plausible?.(event, props ? { props } : undefined);
	} catch {
		/* analytics must never break the page */
	}
}

// Scroll-depth tracker. Fires once per threshold per session.
export function initScrollDepth(): () => void {
	if (!browser || !env.PUBLIC_PLAUSIBLE_DOMAIN) return () => {};
	const thresholds = [25, 50, 75, 100];
	const fired = new Set<number>();

	function onScroll() {
		const h = document.documentElement;
		const scrolled = h.scrollTop + window.innerHeight;
		const pct = Math.min(100, Math.round((scrolled / h.scrollHeight) * 100));
		for (const t of thresholds) {
			if (pct >= t && !fired.has(t)) {
				fired.add(t);
				track('Scroll Depth', { depth: String(t) });
			}
		}
	}

	window.addEventListener('scroll', onScroll, { passive: true });
	return () => window.removeEventListener('scroll', onScroll);
}

// Section visibility tracker. Fires once per section per session when
// the section enters the viewport (50% visible). Uses IntersectionObserver.
export function initSectionVisibility(): () => void {
	if (!browser || !env.PUBLIC_PLAUSIBLE_DOMAIN) return () => {};

	const seen = new Set<string>();

	const observer = new IntersectionObserver(
		(entries) => {
			for (const entry of entries) {
				if (entry.isIntersecting && entry.target instanceof HTMLElement) {
					const id = entry.target.id;
					if (id && !seen.has(id)) {
						seen.add(id);
						track('Section Visible', { section: id });
					}
				}
			}
		},
		{ threshold: 0.3 },
	);

	// Observe all sections with an id attribute
	const sections = document.querySelectorAll('main section[id]');
	sections.forEach((s) => observer.observe(s));

	return () => observer.disconnect();
}

// FAQ expansion tracker. Call from FAQ component on details toggle.
export function trackFaqExpand(question: string): void {
	track('FAQ Expand', { question: question.slice(0, 50) });
}
