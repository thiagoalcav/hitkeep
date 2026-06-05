import { ComponentFixture, TestBed } from '@angular/core/testing';
import { KpiCard } from '@features/analytics/components/kpi-card';

describe('KpiCard', () => {
    let fixture: ComponentFixture<KpiCard>;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [KpiCard]
        }).compileComponents();

        fixture = TestBed.createComponent(KpiCard);
        fixture.componentRef.setInput('label', 'Bounce Rate');
        fixture.componentRef.setInput('value', '45.0%');
        fixture.componentRef.setInput('loading', false);
    });

    it('marks positive deltas as good by default', () => {
        fixture.componentRef.setInput('delta', 10);
        fixture.detectChanges();

        const badge = fixture.nativeElement.querySelector('span.text-xs');
        expect(badge.textContent.trim()).toBe('+10.0%');
        expect(badge.className).toContain('bg-green-100');
    });

    it('marks negative deltas as good when invertDelta is true', () => {
        fixture.componentRef.setInput('delta', -10);
        fixture.componentRef.setInput('invertDelta', true);
        fixture.detectChanges();

        const badge = fixture.nativeElement.querySelector('span.text-xs');
        expect(badge.textContent.trim()).toBe('+10.0%');
        expect(badge.className).toContain('bg-green-100');
    });

    it('highlights the body for live value changes without showing a skeleton', () => {
        fixture.componentRef.setInput('highlight', true);
        fixture.detectChanges();

        const body = fixture.nativeElement.querySelector('.hk-kpi-card__body');
        expect(body.className).toContain('hk-kpi-card__body--highlight');
        expect(fixture.nativeElement.querySelector('p-skeleton')).toBeFalsy();
    });
});
