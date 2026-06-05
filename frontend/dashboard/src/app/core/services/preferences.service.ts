import { Service, effect, signal } from '@angular/core';

@Service()
export class PreferencesService {
    readonly isDarkMode = signal<boolean>(false);

    constructor() {
        if (typeof window !== 'undefined') {
            const saved = localStorage.getItem('hk_theme');
            this.isDarkMode.set(saved === 'dark');

            effect(() => {
                if (this.isDarkMode()) {
                    document.documentElement.classList.add('p-dark');
                    localStorage.setItem('hk_theme', 'dark');
                } else {
                    document.documentElement.classList.remove('p-dark');
                    localStorage.setItem('hk_theme', 'light');
                }
            });
        }
    }

    toggleTheme() {
        this.isDarkMode.update((v) => !v);
    }
}
