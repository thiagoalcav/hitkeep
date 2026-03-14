import { ComponentFixture, TestBed } from "@angular/core/testing";
import { By } from "@angular/platform-browser";
import { TranslocoTestingModule } from "@jsverse/transloco";
import { provideTranslocoLocale } from "@jsverse/transloco-locale";
import { MetricList } from "@features/analytics/components/metric-list";

describe("MetricList", () => {
    let fixture: ComponentFixture<MetricList>;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [
                MetricList,
                TranslocoTestingModule.forRoot({
                    langs: { en: {} },
                    translocoConfig: {
                        availableLangs: ["en"],
                        defaultLang: "en"
                    },
                    preloadLangs: true
                })
            ],
            providers: [
                provideTranslocoLocale({
                    defaultLocale: "en-US",
                    langToLocaleMapping: {
                        en: "en-US",
                        "en-US": "en-US"
                    }
                })
            ]
        }).compileComponents();

        fixture = TestBed.createComponent(MetricList);
        fixture.componentRef.setInput("title", "Top devices");
        fixture.componentRef.setInput("icon", "pi-mobile");
        fixture.componentRef.setInput("data", [
            { name: "Desktop", value: 70 },
            { name: "Mobile", value: 30 }
        ]);
        fixture.detectChanges();
    });

    it("should create", () => {
        expect(fixture.componentInstance).toBeTruthy();
    });

    it("should show distribution percentages for the rows", () => {
        expect(fixture.nativeElement.textContent).toContain("70%");
        expect(fixture.nativeElement.textContent).toContain("30%");
    });

    it("should render distinct device icons", () => {
        fixture.componentRef.setInput("data", [
            { name: "Desktop", value: 70 },
            { name: "Tablet", value: 20 },
            { name: "Mobile", value: 10 }
        ]);
        fixture.detectChanges();

        const icons = fixture.debugElement.queryAll(By.css(".metric-list__item-icon"));
        const classes = icons.map((icon) => icon.nativeElement.className as string);

        expect(classes.some((value) => value.includes("pi-desktop"))).toBeTruthy();
        expect(classes.some((value) => value.includes("pi-tablet"))).toBeTruthy();
        expect(classes.some((value) => value.includes("pi-mobile"))).toBeTruthy();
    });

    it("should not render a leading icon for top pages", () => {
        fixture.componentRef.setInput("icon", "pi-file");
        fixture.componentRef.setInput("linkMode", "path");
        fixture.componentRef.setInput("siteDomain", "example.com");
        fixture.componentRef.setInput("data", [{ name: "/pricing", value: 12 }]);
        fixture.detectChanges();

        expect(fixture.debugElement.query(By.css(".metric-list__item-icon"))).toBeNull();
    });

    it("should render a header view selector when multiple view options are provided", () => {
        fixture.componentRef.setInput("title", "Pages");
        fixture.componentRef.setInput("icon", "pi-file");
        fixture.componentRef.setInput("viewOptions", [
            { label: "Top pages", value: "top" },
            { label: "Landing pages", value: "landing" },
            { label: "Exit pages", value: "exit" }
        ]);
        fixture.componentRef.setInput("selectedView", "landing");
        fixture.detectChanges();

        expect(fixture.debugElement.query(By.css(".metric-list__view-select"))).not.toBeNull();
    });
});
