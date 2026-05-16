import { TestBed } from '@angular/core/testing';
import { vi } from 'vitest';

import { CloudSignupTrackingService } from '@services/cloud-signup-tracking.service';

type HitKeepTestWindow = Window &
    typeof globalThis & {
        hk?: {
            event?: ReturnType<typeof vi.fn>;
        };
    };

describe('CloudSignupTrackingService', () => {
    const testWindow = window as HitKeepTestWindow;
    const originalHk = testWindow.hk;

    beforeEach(() => {
        document.getElementById('hk-cloud-signup-tracker')?.remove();
        testWindow.hk = undefined;
        TestBed.configureTestingModule({});
    });

    afterEach(() => {
        document.getElementById('hk-cloud-signup-tracker')?.remove();
        testWindow.hk = originalHk;
    });

    it('installs a signup-only first-party tracker script', () => {
        const service = TestBed.inject(CloudSignupTrackingService);

        service.install();
        service.install();

        const scripts = document.querySelectorAll('#hk-cloud-signup-tracker');
        expect(scripts.length).toBe(1);

        const script = scripts.item(0) as HTMLScriptElement;
        expect(script.getAttribute('src')).toBe('/hk.js');
        expect(script.getAttribute('data-disable-spa-tracking')).toBe('true');
        expect(script.getAttribute('data-disable-outbound-tracking')).toBe('true');
        expect(script.getAttribute('data-disable-download-tracking')).toBe('true');
        expect(script.getAttribute('data-disable-form-tracking')).toBe('true');
    });

    it('sends tracking events when the tracker is ready', () => {
        const event = vi.fn();
        testWindow.hk = { event };
        const service = TestBed.inject(CloudSignupTrackingService);

        service.trackEvent('signup_started', { jurisdiction: 'EU' });

        expect(event).toHaveBeenCalledWith('signup_started', { jurisdiction: 'EU' });
    });

    it('queues tracking events until the signup tracker loads', () => {
        const service = TestBed.inject(CloudSignupTrackingService);

        service.install();
        service.trackEvent('signup_page_view', { jurisdiction: 'EU' });

        const event = vi.fn();
        testWindow.hk = { event };
        document.getElementById('hk-cloud-signup-tracker')?.dispatchEvent(new Event('load'));

        expect(event).toHaveBeenCalledWith('signup_page_view', { jurisdiction: 'EU' });
    });
});
