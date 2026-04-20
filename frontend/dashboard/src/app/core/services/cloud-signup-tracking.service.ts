import { DOCUMENT } from '@angular/common';
import { Injectable, inject } from '@angular/core';

const CLOUD_SIGNUP_TRACKER_ID = 'hk-cloud-signup-tracker';

@Injectable({ providedIn: 'root' })
export class CloudSignupTrackingService {
    private readonly document = inject(DOCUMENT);

    install(): void {
        if (this.document.getElementById(CLOUD_SIGNUP_TRACKER_ID)) {
            return;
        }

        const script = this.document.createElement('script');
        script.id = CLOUD_SIGNUP_TRACKER_ID;
        script.async = true;
        script.src = '/hk.js';
        script.setAttribute('data-disable-spa-tracking', 'true');
        script.setAttribute('data-disable-outbound-tracking', 'true');
        script.setAttribute('data-disable-download-tracking', 'true');
        script.setAttribute('data-disable-form-tracking', 'true');

        this.document.head.appendChild(script);
    }
}
