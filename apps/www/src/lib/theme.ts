import { browser } from '$app/environment';

const KEY = 'vedox-theme';

export type Theme = 'light' | 'dark';

export function getTheme(): Theme {
	if (!browser) return 'dark';
	const stored = localStorage.getItem(KEY);
	if (stored === 'light' || stored === 'dark') return stored;
	return 'dark';
}

export function setTheme(theme: Theme): void {
	if (!browser) return;
	localStorage.setItem(KEY, theme);
	document.documentElement.setAttribute('data-theme', theme);
}

export function toggleTheme(): Theme {
	const next: Theme = getTheme() === 'dark' ? 'light' : 'dark';
	setTheme(next);
	return next;
}
