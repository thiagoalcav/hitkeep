import { ComponentFixture, TestBed } from '@angular/core/testing';
import { SiteSelector } from '@features/sites/components/site-selector';
import { By } from '@angular/platform-browser';
import { provideHttpClient } from '@angular/common/http';
import { provideRouter } from '@angular/router';
import { TranslocoTestingModule } from '@jsverse/transloco';

describe('SiteSelector', () => {
    let component: SiteSelector;
    let fixture: ComponentFixture<SiteSelector>;

    beforeEach(async () => {
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
            providers: [provideHttpClient(), provideRouter([])]
        }).compileComponents();

        fixture = TestBed.createComponent(SiteSelector);
        component = fixture.componentInstance;
        fixture.componentRef.setInput('sites', [{ id: '1', domain: 'test.com' }]);
        fixture.detectChanges();
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
});
