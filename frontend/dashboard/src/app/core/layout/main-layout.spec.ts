import { ComponentFixture, TestBed } from '@angular/core/testing';
import { MainLayout } from '@layout/main-layout';
import { provideRouter } from '@angular/router';
import { By } from '@angular/platform-browser';

describe('MainLayout', () => {
    let component: MainLayout;
    let fixture: ComponentFixture<MainLayout>;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [MainLayout],
            providers: [provideRouter([])]
        }).compileComponents();

        fixture = TestBed.createComponent(MainLayout);
        component = fixture.componentInstance;
        fixture.detectChanges();
    });

    it('should create', () => {
        expect(component).toBeTruthy();
    });

    it('A11Y: should have correct landmarks', () => {
        const aside = fixture.debugElement.query(By.css('aside'));
        const main = fixture.debugElement.query(By.css('main'));
        const nav = fixture.debugElement.query(By.css('nav'));

        expect(aside).toBeTruthy();
        expect(main).toBeTruthy();
        expect(nav).toBeTruthy();

        // Check labels
        expect(aside.attributes['aria-label']).toBeTruthy();
        expect(main.attributes['role']).toBe('main');
    });

    it('A11Y: buttons should have accessible labels', () => {
        const buttons = fixture.debugElement.queryAll(By.css('button'));
        buttons.forEach((btn) => {
            expect(btn.attributes['aria-label']).withContext('Button missing aria-label').toBeTruthy();
        });
    });
});
