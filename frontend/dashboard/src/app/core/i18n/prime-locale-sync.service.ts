import { Injectable, effect, inject } from '@angular/core';
import { PrimeNG } from 'primeng/config';
import { Translation } from 'primeng/api';
import { TranslocoLocaleService } from '@jsverse/transloco-locale';
import { TranslocoService } from '@jsverse/transloco';
import { injectActiveLang } from '@core/i18n/active-lang';

@Injectable({ providedIn: 'root' })
export class PrimeLocaleSyncService {
    private readonly primeNg = inject(PrimeNG);
    private readonly transloco = inject(TranslocoService);
    private readonly localeService = inject(TranslocoLocaleService);
    private readonly activeLanguage = injectActiveLang();

    constructor() {
        effect(() => {
            this.activeLanguage();
            this.primeNg.setTranslation(this.buildTranslation(this.localeService.getLocale()));
        });
    }

    private buildTranslation(locale: string): Translation {
        return {
            dayNames: this.buildWeekdayNames(locale, 'long'),
            dayNamesShort: this.buildWeekdayNames(locale, 'short'),
            dayNamesMin: this.buildWeekdayNames(locale, 'narrow'),
            monthNames: this.buildMonthNames(locale, 'long'),
            monthNamesShort: this.buildMonthNames(locale, 'short'),
            firstDayOfWeek: this.resolveFirstDayOfWeek(locale),
            dateFormat: this.buildDatePickerDateFormat(locale),
            clear: this.transloco.translate('common.actions.clearAll'),
            apply: this.transloco.translate('common.actions.apply'),
            cancel: this.transloco.translate('common.actions.cancel'),
            chooseDate: this.transloco.translate('common.selectDateRange'),
            am: this.buildDayPeriod(locale, 'am'),
            pm: this.buildDayPeriod(locale, 'pm')
        };
    }

    private buildWeekdayNames(locale: string, width: 'long' | 'short' | 'narrow'): string[] {
        const formatter = new Intl.DateTimeFormat(locale, { weekday: width, timeZone: 'UTC' });
        const sunday = Date.UTC(2026, 0, 4, 12, 0, 0);
        return Array.from({ length: 7 }, (_, index) => formatter.format(new Date(sunday + index * 24 * 60 * 60 * 1000)));
    }

    private buildMonthNames(locale: string, width: 'long' | 'short'): string[] {
        const formatter = new Intl.DateTimeFormat(locale, { month: width, timeZone: 'UTC' });
        return Array.from({ length: 12 }, (_, index) => formatter.format(new Date(Date.UTC(2026, index, 1, 12, 0, 0))));
    }

    private buildDatePickerDateFormat(locale: string): string {
        const parts = new Intl.DateTimeFormat(locale, {
            year: 'numeric',
            month: '2-digit',
            day: '2-digit',
            timeZone: 'UTC'
        }).formatToParts(new Date(Date.UTC(2026, 2, 8, 12, 0, 0)));

        return parts
            .map((part) => {
                switch (part.type) {
                    case 'day':
                        return 'dd';
                    case 'month':
                        return 'mm';
                    case 'year':
                        return 'yy';
                    case 'literal':
                        return part.value;
                    default:
                        return '';
                }
            })
            .join('');
    }

    private resolveFirstDayOfWeek(locale: string): number {
        try {
            const weekInfo = (new Intl.Locale(locale) as Intl.Locale & { weekInfo?: { firstDay?: number } }).weekInfo;
            const firstDay = weekInfo?.firstDay;
            if (typeof firstDay === 'number') {
                return firstDay % 7;
            }
        } catch {
            // Ignore unsupported Intl.Locale weekInfo implementations and use the fallback.
        }

        return locale.toLowerCase().startsWith('en-us') ? 0 : 1;
    }

    private buildDayPeriod(locale: string, dayPeriod: 'am' | 'pm'): string {
        const hour = dayPeriod === 'am' ? 9 : 21;
        const formatter = new Intl.DateTimeFormat(locale, {
            hour: 'numeric',
            hour12: true,
            dayPeriod: 'short',
            timeZone: 'UTC'
        });
        const parts = formatter.formatToParts(new Date(Date.UTC(2026, 2, 8, hour, 0, 0)));
        return parts.find((part) => part.type === 'dayPeriod')?.value ?? dayPeriod.toUpperCase();
    }
}
