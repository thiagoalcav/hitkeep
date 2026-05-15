import { ComponentFixture, TestBed } from '@angular/core/testing';
import { DOCUMENT } from '@angular/common';
import { TranslocoTestingModule } from '@jsverse/transloco';
import { SiteTrackingSettings } from './site-tracking-settings';

describe('SiteTrackingSettings', () => {
    let component: SiteTrackingSettings;
    let fixture: ComponentFixture<SiteTrackingSettings>;
    beforeEach(async () => {
        const document = window.document.implementation.createHTMLDocument('hitkeep tracking test');
        const base = document.createElement('base');
        base.href = '/hitkeep/';
        document.head.append(base);

        await TestBed.configureTestingModule({
            imports: [
                SiteTrackingSettings,
                TranslocoTestingModule.forRoot({
                    langs: { en: {} },
                    translocoConfig: {
                        availableLangs: ['en'],
                        defaultLang: 'en'
                    },
                    preloadLangs: true
                })
            ],
            providers: [{ provide: DOCUMENT, useValue: document }]
        }).compileComponents();

        fixture = TestBed.createComponent(SiteTrackingSettings);
        fixture.componentRef.setInput('site', null);
        component = fixture.componentInstance;
        fixture.detectChanges();
    });
    it('should create', () => {
        expect(component).toBeTruthy();
    });
    it('should update snippet when toggles change', () => {
        const internals = component as SiteTrackingSettings & {
            snippetCode: () => string;
            trackingForm: {
                collectDnt: () => { control: () => { setValue: (value: boolean) => void } };
                disableBeacon: () => { control: () => { setValue: (value: boolean) => void } };
                enableWebVitals: () => { control: () => { setValue: (value: boolean) => void } };
                trackOutbound: () => { control: () => { setValue: (value: boolean) => void } };
                trackDownloads: () => { control: () => { setValue: (value: boolean) => void } };
                trackForms: () => { control: () => { setValue: (value: boolean) => void } };
            };
        };
        const getSnippet = () => internals.snippetCode();

        expect(getSnippet()).toContain('/hitkeep/hk.js');
        expect(getSnippet()).not.toContain('data-collect-dnt');
        expect(getSnippet()).not.toContain('data-disable-beacon');
        expect(getSnippet()).not.toContain('data-enable-web-vitals');
        expect(getSnippet()).not.toContain('data-disable-outbound-tracking');
        expect(getSnippet()).not.toContain('data-disable-download-tracking');
        expect(getSnippet()).not.toContain('data-disable-form-tracking');

        internals.trackingForm.collectDnt().control().setValue(true);
        internals.trackingForm.enableWebVitals().control().setValue(true);
        fixture.detectChanges();
        expect(getSnippet()).toContain('data-collect-dnt="true"');
        expect(getSnippet()).toContain('data-enable-web-vitals="true"');

        internals.trackingForm.disableBeacon().control().setValue(true);
        internals.trackingForm.trackOutbound().control().setValue(false);
        internals.trackingForm.trackDownloads().control().setValue(false);
        internals.trackingForm.trackForms().control().setValue(false);
        fixture.detectChanges();
        expect(getSnippet()).toContain('hk.js');
        expect(getSnippet()).toContain('data-disable-beacon="true"');
        expect(getSnippet()).toContain('data-disable-outbound-tracking="true"');
        expect(getSnippet()).toContain('data-disable-download-tracking="true"');
        expect(getSnippet()).toContain('data-disable-form-tracking="true"');
    });
});
