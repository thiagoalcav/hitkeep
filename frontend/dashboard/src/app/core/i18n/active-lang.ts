import { Signal, inject } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { TranslocoService } from '@jsverse/transloco';
import { map, switchMap } from 'rxjs';

/**
 * Returns a signal that emits the current active language code whenever
 * the language changes and the translation file is fully loaded.
 *
 * Safe to use without null guards: `preloadUserLang()` (APP_INITIALIZER)
 * guarantees the initial language is loaded before any component mounts,
 * so the signal always starts with a non-empty string.
 */
export function injectActiveLang(): Signal<string> {
    const transloco = inject(TranslocoService);
    return toSignal(transloco.langChanges$.pipe(switchMap((lang) => transloco.selectTranslation(lang).pipe(map(() => lang)))), { initialValue: transloco.getActiveLang() }) as Signal<string>;
}
