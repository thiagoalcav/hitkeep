import { ComponentFixture, TestBed } from "@angular/core/testing";
import { provideHttpClient } from "@angular/common/http";
import { provideRouter } from "@angular/router";
import { TranslocoTestingModule } from "@jsverse/transloco";
import { provideTranslocoLocale } from "@jsverse/transloco-locale";

import { AIVisibility } from "@pages/ai-visibility/ai-visibility";

describe("AIVisibility", () => {
    let component: AIVisibility;
    let fixture: ComponentFixture<AIVisibility>;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [
                AIVisibility,
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
                provideHttpClient(),
                provideRouter([]),
                provideTranslocoLocale({
                    defaultLocale: "en-US",
                    langToLocaleMapping: {
                        en: "en-US",
                        "en-US": "en-US"
                    }
                })
            ]
        }).compileComponents();

        fixture = TestBed.createComponent(AIVisibility);
        component = fixture.componentInstance;
        fixture.detectChanges();
    });

    it("should create", () => {
        expect(component).toBeTruthy();
    });
});
