import { TestBed } from '@angular/core/testing';

import { CloudSignupTrackingService } from '@services/cloud-signup-tracking.service';

describe('CloudSignupTrackingService', () => {
    beforeEach(() => {
        document.getElementById('hk-cloud-signup-tracker')?.remove();
        TestBed.configureTestingModule({});
    });

    afterEach(() => {
        document.getElementById('hk-cloud-signup-tracker')?.remove();
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
});
