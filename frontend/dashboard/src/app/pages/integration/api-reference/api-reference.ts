import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormControl, FormsModule, ReactiveFormsModule } from '@angular/forms';
import { toSignal } from '@angular/core/rxjs-interop';
import { finalize } from 'rxjs';
import { TranslocoPipe, TranslocoService } from '@jsverse/transloco';

import { SelectModule } from 'primeng/select';
import { InputTextModule } from 'primeng/inputtext';
import { ButtonModule } from 'primeng/button';

import { PageHeader } from '@components/page-header/page-header';
import { PageBreadcrumb, PageBreadcrumbItem } from '@components/page-breadcrumb/page-breadcrumb';
import { APIDocVersionInfo, APIReferenceService, OpenAPIOperation, OpenAPISpec } from '@services/api-reference.service';

interface OperationView {
    method: string;
    path: string;
    summary: string;
    description: string;
    tags: string[];
    authSchemes: string[];
}

@Component({
    selector: 'app-api-reference-page',
    imports: [CommonModule, FormsModule, ReactiveFormsModule, TranslocoPipe, SelectModule, InputTextModule, ButtonModule, PageHeader, PageBreadcrumb],
    templateUrl: './api-reference.html',
    styleUrl: './api-reference.css',
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class APIReferencePage {
    private docs = inject(APIReferenceService);
    private transloco = inject(TranslocoService);
    private activeLanguage = toSignal(this.transloco.langChanges$, { initialValue: this.transloco.getActiveLang() });

    protected readonly loading = signal(false);
    protected readonly error = signal<string | null>(null);
    protected readonly versions = signal<APIDocVersionInfo[]>([]);
    protected readonly selectedVersion = signal<string | null>(null);
    protected readonly spec = signal<OpenAPISpec | null>(null);
    protected readonly queryControl = new FormControl('', { nonNullable: true });

    protected readonly breadcrumbItems = computed<PageBreadcrumbItem[]>(() => {
        this.activeLanguage();
        return [
            { label: this.transloco.translate('nav.integration'), routerLink: '/integration/api-clients' },
            { label: this.transloco.translate('nav.apiReference'), isCurrent: true }
        ];
    });

    protected readonly versionOptions = computed(() =>
        this.versions().map((version) => ({
            label: `${version.version}${version.latest ? ` (${this.transloco.translate('integration.apiReference.latest')})` : ''}`,
            value: version.version
        }))
    );

    protected readonly authSchemes = computed(() => {
        const schemes = this.spec()?.components?.securitySchemes ?? {};
        return Object.entries(schemes).map(([key, value]) => ({
            key,
            type: value.type,
            description: value.description || ''
        }));
    });

    protected readonly operations = computed<OperationView[]>(() => {
        const spec = this.spec();
        if (!spec?.paths) {
            return [];
        }

        const query = (this.queryControl.value ?? '').trim().toLowerCase();
        const methodOrder = ['get', 'post', 'put', 'patch', 'delete'];
        const list: OperationView[] = [];
        const globalSecurity = spec.security ?? [];

        for (const [path, methods] of Object.entries(spec.paths)) {
            for (const method of methodOrder) {
                const op = methods[method] as OpenAPIOperation | undefined;
                if (!op) continue;

                const summary = op.summary ?? '';
                const description = op.description ?? '';
                const tags = op.tags ?? [];
                const security = op.security ?? globalSecurity;
                const authSchemes = Array.from(new Set(security.flatMap((entry) => Object.keys(entry))));

                const haystack = `${method} ${path} ${summary} ${description} ${tags.join(' ')}`.toLowerCase();
                if (query && !haystack.includes(query)) {
                    continue;
                }

                list.push({
                    method,
                    path,
                    summary: summary || this.transloco.translate('integration.apiReference.noSummary'),
                    description,
                    tags,
                    authSchemes
                });
            }
        }

        return list;
    });

    constructor() {
        this.loadVersions();
    }

    protected refresh(): void {
        const version = this.selectedVersion();
        if (!version) {
            this.loadVersions();
            return;
        }
        this.loadSpec(version);
    }

    protected onVersionChange(version: string | null): void {
        if (!version) {
            return;
        }
        this.selectedVersion.set(version);
        this.loadSpec(version);
    }

    protected formatAuthScheme(scheme: string): string {
        switch (scheme) {
            case 'cookieAuth':
                return this.transloco.translate('integration.apiReference.auth.cookieLabel');
            case 'bearerAuth':
                return this.transloco.translate('integration.apiReference.auth.bearerLabel');
            case 'apiKeyAuth':
                return this.transloco.translate('integration.apiReference.auth.apiKeyLabel');
            default:
                return scheme;
        }
    }

    protected methodClass(method: string): string {
        switch (method.toLowerCase()) {
            case 'get':
                return 'method-get';
            case 'post':
                return 'method-post';
            case 'put':
            case 'patch':
                return 'method-put';
            case 'delete':
                return 'method-delete';
            default:
                return 'method-default';
        }
    }

    private loadVersions(): void {
        this.loading.set(true);
        this.error.set(null);

        this.docs
            .getVersions()
            .subscribe({
                next: (response) => {
                    this.versions.set(response.versions || []);
                    const selected = response.latest || response.versions?.[0]?.version || null;
                    this.selectedVersion.set(selected);
                    if (selected) {
                        this.loadSpec(selected);
                    } else {
                        this.spec.set(null);
                        this.loading.set(false);
                    }
                },
                error: () => {
                    this.error.set('integration.apiReference.errors.loadVersions');
                    this.spec.set(null);
                    this.loading.set(false);
                }
            });
    }

    private loadSpec(version: string): void {
        this.loading.set(true);
        this.error.set(null);

        this.docs
            .getSpec(version)
            .pipe(finalize(() => this.loading.set(false)))
            .subscribe({
                next: (spec) => {
                    this.spec.set(spec);
                },
                error: () => {
                    this.error.set('integration.apiReference.errors.loadSpec');
                    this.spec.set(null);
                }
            });
    }
}
