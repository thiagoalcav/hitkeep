import { ComponentFixture, TestBed } from "@angular/core/testing";
import { provideHttpClient } from "@angular/common/http";
import { provideRouter } from "@angular/router";
import { TranslocoTestingModule } from "@jsverse/transloco";
import { provideTranslocoLocale } from "@jsverse/transloco-locale";

import { AIChatbots } from "@pages/ai-chatbots/ai-chatbots";

describe("AIChatbots", () => {
    let component: AIChatbots;
    let fixture: ComponentFixture<AIChatbots>;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [
                AIChatbots,
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

        fixture = TestBed.createComponent(AIChatbots);
        component = fixture.componentInstance;
        fixture.detectChanges();
    });

    it("should create", () => {
        expect(component).toBeTruthy();
    });
});
