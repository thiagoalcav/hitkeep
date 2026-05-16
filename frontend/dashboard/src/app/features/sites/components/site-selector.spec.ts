import { ComponentFixture, TestBed } from '@angular/core/testing';
import { WritableSignal, signal } from '@angular/core';
import { SiteSelector } from '@features/sites/components/site-selector';
import { By } from '@angular/platform-browser';
import { provideHttpClient } from '@angular/common/http';
import { provideRouter } from '@angular/router';
import { TranslocoTestingModule } from '@jsverse/transloco';
import { vi } from 'vitest';
import { SITE_CAPABILITIES } from '@core/access/capabilities';
import { AccessService } from '@services/access.service';

describe('SiteSelector', () => {
    let component: SiteSelector;
    let fixture: ComponentFixture<SiteSelector>;
    let canSiteMock: ReturnType<typeof vi.fn>;
    let allowedSiteCapabilities: WritableSignal<string[] | null>;

    beforeEach(async () => {
        allowedSiteCapabilities = signal<string[] | null>(null);
        canSiteMock = vi.fn((_siteId: string, capability: string) => allowedSiteCapabilities()?.includes(capability) ?? true);

        await TestBed.configureTestingModule({
            imports: [
                SiteSelector,
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
                provideRouter([]),
                {
                    provide: AccessService,
                    useValue: {
                        canSite: canSiteMock
                    }
                }
            ]
        }).compileComponents();

        fixture = TestBed.createComponent(SiteSelector);
        component = fixture.componentInstance;
        fixture.componentRef.setInput('sites', [{ id: '1', domain: 'test.com' }]);
        fixture.componentRef.setInput('current', { id: '1', domain: 'test.com' });
        fixture.detectChanges();
    });

    afterEach(() => {
        TestBed.resetTestingModule();
    });

    it('should create', () => {
        expect(component).toBeTruthy();
    });

    it('A11Y: should have a label associated with the dropdown', () => {
        const label = fixture.debugElement.query(By.css('label'));
        const select = fixture.debugElement.query(By.css('p-select'));
        expect(label.nativeElement.getAttribute('for')).toBe('site-dropdown');
        expect(select.attributes['inputId']).toBe('site-dropdown');
    });

    it('A11Y: Add Site button should have aria-label', () => {
        const btn = fixture.debugElement.query(By.css('button[aria-label]'));
        expect(btn).toBeTruthy();
    });

    it('disables dashboard sharing when the active site cannot manage team access', () => {
        allowedSiteCapabilities.set([SITE_CAPABILITIES.view]);
        fixture.detectChanges();

        const shareButton = fixture.debugElement.query(By.css('button[title="sites.selector.shareDashboardAria"]'));

        expect(shareButton.nativeElement.disabled).toBe(true);
    });
});
