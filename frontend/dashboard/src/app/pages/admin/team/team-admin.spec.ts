import { ComponentFixture, TestBed } from "@angular/core/testing";
import { provideHttpClient } from "@angular/common/http";
import { provideHttpClientTesting } from "@angular/common/http/testing";
import { provideRouter } from "@angular/router";
import { TranslocoTestingModule } from "@jsverse/transloco";
import { TeamAdminPage } from "./team-admin";

describe("TeamAdminPage", () => {
    let component: TeamAdminPage;
    let fixture: ComponentFixture<TeamAdminPage>;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [
                TeamAdminPage,
                TranslocoTestingModule.forRoot({
                    langs: { en: {} },
                    translocoConfig: {
                        availableLangs: ["en"],
                        defaultLang: "en"
                    },
                    preloadLangs: true
                })
            ],
            providers: [provideHttpClient(), provideHttpClientTesting(), provideRouter([])]
        }).compileComponents();

        fixture = TestBed.createComponent(TeamAdminPage);
        component = fixture.componentInstance;
        fixture.detectChanges();
    });

    it("should create", () => {
        expect(component).toBeTruthy();
    });
});
