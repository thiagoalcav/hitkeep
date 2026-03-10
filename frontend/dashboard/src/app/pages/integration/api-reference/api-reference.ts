import { ChangeDetectionStrategy, Component, OnDestroy, OnInit, computed, effect, inject, signal } from "@angular/core";
import { toSignal } from "@angular/core/rxjs-interop";
import { TranslocoPipe, TranslocoService } from "@jsverse/transloco";
import { DomSanitizer, SafeResourceUrl } from "@angular/platform-browser";

import { PageBreadcrumb, PageBreadcrumbItem } from "@components/page-breadcrumb/page-breadcrumb";
import { PageHeader } from "@components/page-header/page-header";
import { PreferencesService } from "@services/preferences.service";

interface ScalarFrameEvent {
    source?: string;
    type?: "ready" | "error";
}

const SCALAR_FRAME_EVENT_SOURCE = "hitkeep-scalar-frame";

@Component({
    selector: "app-api-reference-page",
    imports: [PageHeader, PageBreadcrumb, TranslocoPipe],
    templateUrl: "./api-reference.html",
    styleUrl: "./api-reference.css",
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class APIReferencePage implements OnInit, OnDestroy {
    private transloco = inject(TranslocoService);
    private prefs = inject(PreferencesService);
    private sanitizer = inject(DomSanitizer);
    private activeLanguage = toSignal(this.transloco.langChanges$, { initialValue: this.transloco.getActiveLang() });
    private handleFrameMessage = (event: MessageEvent<ScalarFrameEvent>): void => {
        if (typeof window === "undefined" || event.origin !== window.location.origin) {
            return;
        }

        if (event.data?.source !== SCALAR_FRAME_EVENT_SOURCE) {
            return;
        }

        if (event.data.type === "ready") {
            this.error.set(null);
            this.viewerReady.set(true);
            return;
        }

        if (event.data.type === "error") {
            this.viewerReady.set(true);
            this.error.set("integration.apiReference.errors.loadSpec");
        }
    };

    protected readonly viewerReady = signal(false);
    protected readonly error = signal<string | null>(null);
    protected readonly specUrl = "/api/docs/v1/openapi.json";
    protected readonly scalarFrameSrc = computed<SafeResourceUrl>(() => {
        const theme = this.prefs.isDarkMode() ? "dark" : "light";
        const query = new URLSearchParams({
            spec: this.specUrl,
            theme,
            hideThemeToggle: "1",
            agent: "0",
            withDefaultFonts: "0",
            hideClientButton: "1",
            hiddenClients: "1",
            telemetry: "0"
        });

        const frameUrl = `/scalar/index.html?${query.toString()}`;
        return this.sanitizer.bypassSecurityTrustResourceUrl(frameUrl);
    });

    protected readonly breadcrumbItems = computed<PageBreadcrumbItem[]>(() => {
        this.activeLanguage();
        return [
            { label: this.transloco.translate("nav.integration"), routerLink: "/integration/api-clients" },
            { label: this.transloco.translate("nav.apiReference"), isCurrent: true }
        ];
    });

    constructor() {
        effect(() => {
            this.prefs.isDarkMode();
            this.viewerReady.set(false);
            this.error.set(null);
        });
    }

    ngOnInit(): void {
        if (typeof window === "undefined") {
            return;
        }

        window.addEventListener("message", this.handleFrameMessage);
    }

    ngOnDestroy(): void {
        if (typeof window === "undefined") {
            return;
        }

        window.removeEventListener("message", this.handleFrameMessage);
    }

    protected onFrameError(): void {
        this.viewerReady.set(false);
        this.error.set("integration.apiReference.errors.loadSpec");
    }

    protected onFrameLoad(): void {
        if (!this.error()) {
            this.viewerReady.set(true);
        }
    }
}
