import { DOCUMENT } from '@angular/common';
import { AfterViewInit, ChangeDetectionStrategy, Component, ElementRef, OnDestroy, ViewChild, computed, inject, signal } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { TranslocoPipe, TranslocoService } from '@jsverse/transloco';

import { PageBreadcrumb, PageBreadcrumbItem } from '@components/page-breadcrumb/page-breadcrumb';
import { PageHeader } from '@components/page-header/page-header';

const SCALAR_REFERENCE_ID = 'api-reference';
const SCALAR_SCRIPT_ID = 'hitkeep-scalar-script';
const SCALAR_SCRIPT_URL = 'https://cdn.jsdelivr.net/npm/@scalar/api-reference';

interface ScalarGlobal {
    createApiReference: (mountTarget: string | Element, configuration: { url: string; proxyUrl?: string; agent?: { disabled?: boolean } }) => unknown;
}

@Component({
    selector: 'app-api-reference-page',
    imports: [PageHeader, PageBreadcrumb, TranslocoPipe],
    templateUrl: './api-reference.html',
    styleUrl: './api-reference.css',
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class APIReferencePage implements AfterViewInit, OnDestroy {
    @ViewChild('referenceContainer')
    private referenceContainer?: ElementRef<HTMLDivElement>;

    private document = inject(DOCUMENT);
    private transloco = inject(TranslocoService);
    private activeLanguage = toSignal(this.transloco.langChanges$, { initialValue: this.transloco.getActiveLang() });

    protected readonly viewerReady = signal(false);
    protected readonly error = signal<string | null>(null);
    protected readonly specUrl = '/api/docs/v1/openapi.json';

    protected readonly breadcrumbItems = computed<PageBreadcrumbItem[]>(() => {
        this.activeLanguage();
        return [
            { label: this.transloco.translate('nav.integration'), routerLink: '/integration/api-clients' },
            { label: this.transloco.translate('nav.apiReference'), isCurrent: true }
        ];
    });

    ngAfterViewInit(): void {
        void this.loadViewer();
    }

    ngOnDestroy(): void {
        const container = this.referenceContainer?.nativeElement;
        if (container) {
            container.replaceChildren();
        }
    }

    private async loadViewer(): Promise<void> {
        this.error.set(null);
        this.viewerReady.set(false);

        const container = this.referenceContainer?.nativeElement;
        if (!container) {
            this.error.set('integration.apiReference.errors.loadSpec');
            return;
        }

        try {
            await this.ensureScript();
            const scalar = this.getScalarGlobal();
            if (!scalar) {
                this.error.set('integration.apiReference.errors.loadSpec');
                return;
            }

            this.mountReference(container, scalar);
            this.viewerReady.set(true);
        } catch {
            this.viewerReady.set(false);
            this.error.set('integration.apiReference.errors.loadSpec');
        }
    }

    private ensureScript(): Promise<void> {
        if (this.getScalarGlobal()) {
            return Promise.resolve();
        }

        const existing = this.document.getElementById(SCALAR_SCRIPT_ID) as HTMLScriptElement | null;
        if (existing) {
            return new Promise<void>((resolve, reject) => {
                existing.addEventListener('load', () => resolve(), { once: true });
                existing.addEventListener('error', () => reject(new Error('Failed to load Scalar script.')), { once: true });
            });
        }

        return new Promise<void>((resolve, reject) => {
            const script = this.document.createElement('script');
            script.id = SCALAR_SCRIPT_ID;
            script.src = SCALAR_SCRIPT_URL;
            script.async = true;
            script.onload = () => resolve();
            script.onerror = () => reject(new Error('Failed to load Scalar script.'));
            this.document.body.appendChild(script);
        });
    }

    private mountReference(container: HTMLDivElement, scalar: ScalarGlobal): void {
        container.replaceChildren();
        container.id = SCALAR_REFERENCE_ID;
        scalar.createApiReference(container, {
            url: this.specUrl,
            agent: {
                disabled: true
            }
        });
    }

    private getScalarGlobal(): ScalarGlobal | null {
        const globalRef = (this.document.defaultView as (Window & { Scalar?: ScalarGlobal }) | null)?.Scalar;
        if (!globalRef || typeof globalRef.createApiReference !== 'function') {
            return null;
        }
        return globalRef;
    }
}
