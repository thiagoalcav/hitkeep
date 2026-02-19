import { ComponentFixture, TestBed } from '@angular/core/testing';
import { TranslocoTestingModule } from '@jsverse/transloco';
import { SiteTrackingSettings } from './site-tracking-settings';

describe('SiteTrackingSettings', () => {
    let component: SiteTrackingSettings;
    let fixture: ComponentFixture<SiteTrackingSettings>;
    beforeEach(async () => {
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
            ]
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
            };
        };
        const getSnippet = () => internals.snippetCode();

        expect(getSnippet()).toContain('hk.js');
        expect(getSnippet()).not.toContain('data-collect-dnt');
        expect(getSnippet()).not.toContain('data-disable-beacon');

        internals.trackingForm.collectDnt().control().setValue(true);
        fixture.detectChanges();
        expect(getSnippet()).toContain('data-collect-dnt="true"');

        internals.trackingForm.disableBeacon().control().setValue(true);
        fixture.detectChanges();
        expect(getSnippet()).toContain('hk.js');
        expect(getSnippet()).toContain('data-disable-beacon="true"');
    });
});
