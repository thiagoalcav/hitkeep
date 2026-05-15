import { ComponentFixture, TestBed } from '@angular/core/testing';
import { afterEach } from 'vitest';
import { Brand } from './brand';

describe('Brand', () => {
    let fixture: ComponentFixture<Brand>;
    let base: HTMLBaseElement;

    beforeEach(async () => {
        base = document.querySelector('base') ?? document.createElement('base');
        base.href = '/hitkeep/';
        if (!base.parentNode) {
            document.head.append(base);
        }

        await TestBed.configureTestingModule({
            imports: [Brand]
        }).compileComponents();

        fixture = TestBed.createComponent(Brand);
        fixture.detectChanges();
    });

    afterEach(() => {
        base.href = '/';
    });

    it('loads the logo from the configured browser base path', () => {
        const component = fixture.componentInstance as unknown as { iconUrl: () => string };

        expect(component.iconUrl()).toBe('/hitkeep/icon.png');
    });
});
