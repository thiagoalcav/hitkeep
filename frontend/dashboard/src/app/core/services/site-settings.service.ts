import { Injectable, signal, computed } from "@angular/core";

@Injectable({ providedIn: "root" })
export class SiteSettingsService {
    private readonly requestedTab = signal<string | null>(null);
    readonly request = computed(() => this.requestedTab());

    open(tab = "0") {
        this.requestedTab.set(tab);
    }

    clear() {
        this.requestedTab.set(null);
    }
}
