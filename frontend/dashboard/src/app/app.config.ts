import { ApplicationConfig, isDevMode, provideBrowserGlobalErrorListeners, provideZonelessChangeDetection } from '@angular/core';
import { provideRouter } from '@angular/router';
import { providePrimeNG } from 'primeng/config';
import Aura from '@primeuix/themes/aura';

import { routes } from './app.routes';
import { provideHttpClient, withFetch, withInterceptors } from '@angular/common/http';
import { authInterceptor } from '@core/interceptors/auth.interceptor';
import { shareInterceptor } from '@core/interceptors/share.interceptor';
import { provideTransloco } from '@jsverse/transloco';
import { provideTranslocoLocale } from '@jsverse/transloco-locale';
import { TranslocoHttpLoader } from './transloco-loader';
import { providePreloadUserLang } from '@core/i18n/preload-user-lang';

export const appConfig: ApplicationConfig = {
    providers: [
        provideBrowserGlobalErrorListeners(),
        provideZonelessChangeDetection(),
        provideHttpClient(withFetch(), withInterceptors([shareInterceptor, authInterceptor])),
        provideRouter(routes),
        providePrimeNG({
            theme: {
                preset: Aura,
                options: { darkModeSelector: '.p-dark' }
            }
        }),
        provideTransloco({
            config: {
                availableLangs: ['en', 'de'],
                defaultLang: 'en',
                fallbackLang: 'en',
                reRenderOnLangChange: true,
                prodMode: !isDevMode()
            },
            loader: TranslocoHttpLoader
        }),
        provideTranslocoLocale({
            defaultLocale: 'en-US',
            langToLocaleMapping: {
                en: 'en-US',
                de: 'de-DE',
                'en-US': 'en-US',
                'de-DE': 'de-DE'
            }
        }),
        providePreloadUserLang()
    ]
};
