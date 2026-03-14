import { NgOptimizedImage } from "@angular/common";
import { Component, computed, input } from "@angular/core";

@Component({
    selector: "app-brand",
    standalone: true,
    imports: [NgOptimizedImage],
    template: `
        <div class="flex items-center gap-3 select-none">
            <img ngSrc="/icon.png" alt="HitKeep Logo" class="object-cover" [class]="imgClass()" [width]="imgSize()" [height]="imgSize()" priority />
            <span class="font-bold tracking-tight text-[var(--p-text-color)]" [class]="textClass()"> HitKeep </span>
        </div>
    `
})
export class Brand {
    size = input<"small" | "large">("small");

    protected imgSize = computed(() => {
        return this.size() === "large" ? 48 : 32;
    });

    protected imgClass = computed(() => {
        return this.size() === "large" ? "w-12 h-12" : "w-8 h-8";
    });

    protected textClass = computed(() => {
        return this.size() === "large" ? "text-3xl" : "text-xl";
    });
}
