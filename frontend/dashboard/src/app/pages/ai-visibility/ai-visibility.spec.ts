import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { provideRouter } from '@angular/router';
import { TranslocoTestingModule } from '@jsverse/transloco';
import { provideTranslocoLocale } from '@jsverse/transloco-locale';

import { AIVisibility } from '@pages/ai-visibility/ai-visibility';
import { SiteService } from '@features/sites/services/site.service';

describe('AIVisibility', () => {
    let component: AIVisibility;
    let fixture: ComponentFixture<AIVisibility>;
    let httpMock: HttpTestingController;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [
                AIVisibility,
                TranslocoTestingModule.forRoot({
                    langs: { en: {} },
                    translocoConfig: {
                        availableLangs: ['en'],
                        defaultLang: 'en'
                    },
                    preloadLangs: true
                })
            ],
            providers: [
                provideHttpClient(),
                provideHttpClientTesting(),
                provideRouter([]),
                provideTranslocoLocale({
                    defaultLocale: 'en-US',
                    langToLocaleMapping: {
                        en: 'en-US',
                        'en-US': 'en-US'
                    }
                })
            ]
        }).compileComponents();

        httpMock = TestBed.inject(HttpTestingController);
        fixture = TestBed.createComponent(AIVisibility);
        component = fixture.componentInstance;
        fixture.detectChanges();
    });

    afterEach(() => {
        httpMock.verify();
    });

    it('should create', () => {
        expect(component).toBeTruthy();
    });

    it('should expose correlation tab counts from the loaded report', () => {
        const instance = component as AIVisibility & {
            correlation: { set(value: unknown): void };
            correlationTabs: () => { value: string; count: number }[];
        };

        instance.correlation.set({
            summary: {
                total_fetches: 0,
                fetched_paths: 0,
                correlated_paths: 0,
                ai_referred_visits: 0,
                uncorrelated_fetches: 0
            },
            citation_yield: [{ path: '/docs', assistant_name: 'GPTBot', fetch_count: 12, ai_referred_visits: 4, citation_yield_pct: 33.3 }],
            opportunity_pages: [{ path: '/pricing', fetch_count: 21, ai_referred_visits: 0, error_requests: 1, error_rate_pct: 4.8 }],
            failure_hotspots: [{ assistant_name: 'ClaudeBot', path_prefix: '/api', total_requests: 9, error_requests: 2, error_rate_pct: 22.2 }]
        });

        const tabs = instance.correlationTabs();

        expect(tabs.length).toBe(3);
        expect(tabs[0]?.value).toBe('citationYield');
        expect(tabs[0]?.count).toBe(1);
        expect(tabs[1]?.value).toBe('opportunityPages');
        expect(tabs[1]?.count).toBe(1);
        expect(tabs[2]?.value).toBe('failureHotspots');
        expect(tabs[2]?.count).toBe(1);
    });

    it('should map error rates to visual severities', () => {
        const instance = component as AIVisibility & { errorRateSeverity(value: number): string };

        expect(instance.errorRateSeverity(0)).toBe('secondary');
        expect(instance.errorRateSeverity(4.9)).toBe('warn');
        expect(instance.errorRateSeverity(5)).toBe('danger');
    });

    it('should link to the AI fetch setup guide from the toolbar', () => {
        const setupGuideLink = fixture.nativeElement.querySelector('a[href="https://hitkeep.com/guides/tracking/ai-fetch-ingest/"]');

        expect(setupGuideLink).not.toBeNull();
        expect(setupGuideLink.getAttribute('target')).toBe('_blank');
        expect(setupGuideLink.textContent).toContain('aiVisibility.docsAction');
    });

    it('should not render a setup-required callout when a site is selected but no AI fetches exist', () => {
        const siteService = TestBed.inject(SiteService);
        siteService.activeSite.set({
            id: 'site-1',
            domain: 'example.com',
            created_at: '',
            user_id: 'user-1'
        });

        fixture.detectChanges();

        httpMock
            .expectOne((req) => req.url === '/api/sites/site-1/ai-fetch/overview')
            .flush({
                total_requests: 0,
                unique_paths: 0,
                unique_assistants: 0,
                error_rate_4xx: 0,
                error_rate_5xx: 0,
                median_response_ms: 0,
                total_bytes: 0,
                top_assistants: [],
                top_families: [],
                top_paths: [],
                top_error_paths: [],
                resource_type_split: []
            });
        httpMock.expectOne((req) => req.url === '/api/sites/site-1/ai-fetch/timeseries').flush([]);
        httpMock
            .expectOne((req) => req.url === '/api/sites/site-1/ai-fetch/correlation')
            .flush({
                summary: {
                    total_fetches: 0,
                    fetched_paths: 0,
                    correlated_paths: 0,
                    ai_referred_visits: 0,
                    uncorrelated_fetches: 0
                },
                citation_yield: [],
                opportunity_pages: [],
                failure_hotspots: []
            });

        fixture.detectChanges();

        expect(fixture.nativeElement.textContent).not.toContain('Server-side fetch capture required');
    });
});
