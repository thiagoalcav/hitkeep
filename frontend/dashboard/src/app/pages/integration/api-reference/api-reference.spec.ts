import { SecurityContext } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { provideNoopAnimations } from '@angular/platform-browser/animations';
import { DOCUMENT } from '@angular/common';
import { DomSanitizer } from '@angular/platform-browser';
import { TranslocoTestingModule } from '@jsverse/transloco';
import { APIReferencePage } from './api-reference';
import { PreferencesService } from '@services/preferences.service';

describe('APIReferencePage', () => {
    let fixture: ComponentFixture<APIReferencePage>;
    let sanitizer: DomSanitizer;

    beforeEach(async () => {
        const document = window.document.implementation.createHTMLDocument('hitkeep api reference test');
        const base = document.createElement('base');
        base.href = '/hitkeep/';
        document.head.append(base);

        await TestBed.configureTestingModule({
            imports: [
                APIReferencePage,
                TranslocoTestingModule.forRoot({
                    langs: {
                        en: {
                            nav: {
                                integration: 'Integration',
                                apiReference: 'API Reference'
                            },
                            integration: {
                                apiReference: {
                                    subtitle: 'Versioned REST API reference',
                                    loading: 'Loading...'
                                }
                            }
                        }
                    },
                    translocoConfig: {
                        availableLangs: ['en'],
                        defaultLang: 'en'
                    },
                    preloadLangs: true
                })
            ],
            providers: [
                provideNoopAnimations(),
                provideRouter([]),
                {
                    provide: PreferencesService,
                    useValue: {
                        isDarkMode: () => false
                    }
                },
                { provide: DOCUMENT, useValue: document }
            ]
        }).compileComponents();

        sanitizer = TestBed.inject(DomSanitizer);
        fixture = TestBed.createComponent(APIReferencePage);
        fixture.detectChanges();
    });

    it('uses the supported Scalar query parameters', () => {
        const frameUrl = sanitizer.sanitize(SecurityContext.RESOURCE_URL, fixture.componentInstance['scalarFrameSrc']());
        expect(frameUrl).toBeTruthy();
        const url = new URL(frameUrl!, 'https://example.test');

        expect(url.pathname).toBe('/hitkeep/scalar/index.html');
        expect(url.searchParams.get('spec')).toBe('/hitkeep/api/docs/v1/openapi.json');
        expect(url.searchParams.get('withDefaultFonts')).toBe('0');
        expect(url.searchParams.get('hideClientButton')).toBe('1');
        expect(url.searchParams.get('hiddenClients')).toBe('1');
        expect(url.searchParams.get('telemetry')).toBe('0');
        expect(url.searchParams.get('defaultFonts')).toBeNull();
        expect(url.searchParams.get('showClient')).toBeNull();
    });
});
