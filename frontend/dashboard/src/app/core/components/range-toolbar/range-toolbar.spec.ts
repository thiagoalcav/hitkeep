import { Signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { TranslocoService, TranslocoTestingModule } from '@jsverse/transloco';
import { provideTranslocoLocale } from '@jsverse/transloco-locale';
import { PrimeNG } from 'primeng/config';

import { DEFAULT_RANGE_OPTIONS, RangeOption, RangeToolbar } from './range-toolbar';
import { PrimeLocaleSyncService } from '@core/i18n/prime-locale-sync.service';

describe('RangeToolbar', () => {
    let fixture: ComponentFixture<RangeToolbar>;
    let component: RangeToolbar;
    let transloco: TranslocoService;
    let primeNg: PrimeNG;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [
                RangeToolbar,
                TranslocoTestingModule.forRoot({
                    langs: {
                        en: {
                            common: {
                                timeRanges: {
                                    last24Hours: 'Last 24 hours',
                                    last7Days: 'Last 7 days',
                                    last30Days: 'Last 30 days',
                                    lastYear: 'Last year',
                                    customRange: 'Custom range',
                                    customShort: 'Custom'
                                },
                                actions: {
                                    refresh: 'Refresh'
                                },
                                timeRangeSelectorAria: 'Select range',
                                refreshDataTooltip: 'Refresh'
                            }
                        },
                        de: {
                            common: {
                                timeRanges: {
                                    last24Hours: 'Letzte 24 Stunden',
                                    last7Days: 'Letzte 7 Tage',
                                    last30Days: 'Letzte 30 Tage',
                                    lastYear: 'Letztes Jahr',
                                    customRange: 'Benutzerdefiniert',
                                    customShort: 'Benutzerdef.'
                                },
                                actions: {
                                    refresh: 'Aktualisieren'
                                },
                                timeRangeSelectorAria: 'Zeitraum auswählen',
                                refreshDataTooltip: 'Aktualisieren'
                            }
                        }
                    },
                    translocoConfig: {
                        availableLangs: ['en', 'de'],
                        defaultLang: 'en'
                    },
                    preloadLangs: true
                })
            ],
            providers: [
                PrimeLocaleSyncService,
                provideTranslocoLocale({
                    defaultLocale: 'en-US',
                    langToLocaleMapping: {
                        en: 'en-US',
                        de: 'de-DE'
                    }
                })
            ]
        }).compileComponents();

        transloco = TestBed.inject(TranslocoService);
        primeNg = TestBed.inject(PrimeNG);
        TestBed.inject(PrimeLocaleSyncService);
        fixture = TestBed.createComponent(RangeToolbar);
        component = fixture.componentInstance;
        fixture.componentRef.setInput('timeRanges', DEFAULT_RANGE_OPTIONS);
        fixture.componentRef.setInput('selectedRange', DEFAULT_RANGE_OPTIONS[2] as RangeOption);
        fixture.detectChanges();
    });

    const translatedLabels = (toolbar: RangeToolbar) => {
        const { translatedTimeRanges } = toolbar as unknown as { translatedTimeRanges: Signal<RangeOption[]> };
        return translatedTimeRanges().map((option) => option.label);
    };

    const datePickerFormat = (toolbar: RangeToolbar) => {
        const { datePickerDateFormat } = toolbar as unknown as { datePickerDateFormat: Signal<string> };
        return datePickerDateFormat();
    };

    const datePickerHourFormat = (toolbar: RangeToolbar) => {
        const { datePickerHourFormat } = toolbar as unknown as { datePickerHourFormat: Signal<'12' | '24'> };
        return datePickerHourFormat();
    };

    it('translates default ranges from the active language', () => {
        expect(translatedLabels(component)).toEqual(['Last 24 hours', 'Last 7 days', 'Last 30 days', 'Last year', 'Custom range']);
    });

    it('updates translated labels when the active language changes', async () => {
        transloco.setActiveLang('de');
        fixture.detectChanges();
        await fixture.whenStable();

        expect(translatedLabels(component)).toEqual(['Letzte 24 Stunden', 'Letzte 7 Tage', 'Letzte 30 Tage', 'Letztes Jahr', 'Benutzerdefiniert']);
    });

    it('uses the active locale for the custom date picker format', async () => {
        expect(datePickerFormat(component)).toBe('mm/dd/yy');
        expect(datePickerHourFormat(component)).toBe('12');

        transloco.setActiveLang('de');
        fixture.detectChanges();
        await fixture.whenStable();

        expect(datePickerFormat(component)).toBe('dd.mm.yy');
        expect(datePickerHourFormat(component)).toBe('24');
    });

    it('syncs PrimeNG calendar translations with the active locale', async () => {
        expect(primeNg.translation.dayNames?.[0]).toBe('Sunday');
        expect(primeNg.translation.monthNames?.[0]).toBe('January');
        expect(primeNg.translation.firstDayOfWeek).toBe(0);

        transloco.setActiveLang('de');
        fixture.detectChanges();
        await fixture.whenStable();

        expect(primeNg.translation.dayNames?.[0]).toBe('Sonntag');
        expect(primeNg.translation.monthNames?.[0]).toBe('Januar');
        expect(primeNg.translation.firstDayOfWeek).toBe(1);
    });
});
