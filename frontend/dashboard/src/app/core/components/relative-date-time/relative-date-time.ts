import { ChangeDetectionStrategy, Component, computed, effect, inject, input, signal } from "@angular/core";
import { toSignal } from "@angular/core/rxjs-interop";
import { TranslocoService } from "@jsverse/transloco";
import { TranslocoLocaleService } from "@jsverse/transloco-locale";

type RelativeDateKind = "date" | "datetime";
type RelativeUnit = "year" | "month" | "week" | "day" | "hour" | "minute" | "second";

const SECOND_IN_MS = 1000;
const MINUTE_IN_MS = 60 * SECOND_IN_MS;
const HOUR_IN_MS = 60 * MINUTE_IN_MS;
const DAY_IN_MS = 24 * HOUR_IN_MS;
const WEEK_IN_MS = 7 * DAY_IN_MS;
const MONTH_IN_MS = 30 * DAY_IN_MS;
const YEAR_IN_MS = 365 * DAY_IN_MS;
const DEFAULT_REFRESH_INTERVAL_MS = 60 * SECOND_IN_MS;
const MIN_REFRESH_INTERVAL_MS = SECOND_IN_MS;

const DATE_RELATIVE_UNITS: readonly { unit: RelativeUnit; milliseconds: number }[] = [
    { unit: "year", milliseconds: YEAR_IN_MS },
    { unit: "month", milliseconds: MONTH_IN_MS },
    { unit: "week", milliseconds: WEEK_IN_MS },
    { unit: "day", milliseconds: DAY_IN_MS }
];

const DATETIME_RELATIVE_UNITS: readonly { unit: RelativeUnit; milliseconds: number }[] = [
    ...DATE_RELATIVE_UNITS,
    { unit: "hour", milliseconds: HOUR_IN_MS },
    { unit: "minute", milliseconds: MINUTE_IN_MS },
    { unit: "second", milliseconds: SECOND_IN_MS }
];

@Component({
    selector: "app-relative-date-time",
    templateUrl: "./relative-date-time.html",
    styleUrl: "./relative-date-time.css",
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class RelativeDateTime {
    private readonly transloco = inject(TranslocoService);
    private readonly localeService = inject(TranslocoLocaleService);
    private readonly activeLang = toSignal(this.transloco.langChanges$, {
        initialValue: this.transloco.getActiveLang()
    });
    private readonly activeLocale = toSignal(this.localeService.localeChanges$, {
        initialValue: this.localeService.getLocale()
    });
    private readonly now = signal(Date.now());

    readonly value = input<string | number | Date | null | undefined>(null);
    readonly kind = input<RelativeDateKind>("datetime");
    readonly emptyText = input("-");
    readonly refreshIntervalMs = input(DEFAULT_REFRESH_INTERVAL_MS);

    protected readonly parsedDate = computed(() => this.parseDate(this.value()));
    protected readonly isoDateTime = computed(() => this.parsedDate()?.toISOString() ?? null);
    protected readonly locale = computed(() => {
        this.activeLang();
        return this.activeLocale();
    });
    protected readonly absoluteLabel = computed(() => {
        const parsedDate = this.parsedDate();
        if (!parsedDate) {
            return "";
        }

        const locale = this.locale();
        if (this.kind() === "date") {
            return this.localeService.localizeDate(parsedDate, locale, {
                dateStyle: "medium"
            });
        }

        return this.localeService.localizeDate(parsedDate, locale, {
            dateStyle: "medium",
            timeStyle: "short"
        });
    });
    protected readonly relativeLabel = computed(() => {
        const parsedDate = this.parsedDate();
        if (!parsedDate) {
            return this.emptyText();
        }

        const formatter = new Intl.RelativeTimeFormat(this.locale(), {
            numeric: "auto",
            style: "long"
        });

        return this.formatRelative(parsedDate.getTime() - this.now(), formatter);
    });
    protected readonly ariaLabel = computed(() => {
        const parsedDate = this.parsedDate();
        if (!parsedDate) {
            return this.emptyText();
        }

        const relative = this.relativeLabel();
        const absolute = this.absoluteLabel();
        return absolute ? `${relative}. ${absolute}` : relative;
    });

    constructor() {
        effect((onCleanup) => {
            if (typeof window === "undefined") {
                return;
            }

            const intervalMs = Math.max(MIN_REFRESH_INTERVAL_MS, this.refreshIntervalMs());
            const timerId = window.setInterval(() => {
                this.now.set(Date.now());
            }, intervalMs);
            onCleanup(() => {
                window.clearInterval(timerId);
            });
        });
    }

    private formatRelative(diffMs: number, formatter: Intl.RelativeTimeFormat): string {
        const units = this.kind() === "date" ? DATE_RELATIVE_UNITS : DATETIME_RELATIVE_UNITS;
        const absoluteDiff = Math.abs(diffMs);
        const closestUnit = units.find((unit) => absoluteDiff >= unit.milliseconds) ?? units[units.length - 1];
        const rounded = Math.round(diffMs / closestUnit.milliseconds);
        const amount = Object.is(rounded, -0) ? 0 : rounded;
        return formatter.format(amount, closestUnit.unit);
    }

    private parseDate(value: string | number | Date | null | undefined): Date | null {
        if (value === null || value === undefined || value === "") {
            return null;
        }

        const parsedDate = value instanceof Date ? value : new Date(value);
        if (Number.isNaN(parsedDate.getTime())) {
            return null;
        }

        return parsedDate;
    }
}
