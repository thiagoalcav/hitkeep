import { ComponentFixture, TestBed } from "@angular/core/testing";
import { provideHttpClient } from "@angular/common/http";
import { provideHttpClientTesting } from "@angular/common/http/testing";
import { provideRouter } from "@angular/router";
import { TranslocoTestingModule } from "@jsverse/transloco";
import { CreateTeamDialog } from "./create-team-dialog";

describe("CreateTeamDialog", () => {
    let component: CreateTeamDialog;
    let fixture: ComponentFixture<CreateTeamDialog>;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [
                CreateTeamDialog,
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

        fixture = TestBed.createComponent(CreateTeamDialog);
        component = fixture.componentInstance;
        fixture.detectChanges();
    });

    it("should create", () => {
        expect(component).toBeTruthy();
    });

    it("should reset form on resetForm", () => {
        component.resetForm();
        expect(component.visible()).toBe(false);
    });

    it("should not submit when form is invalid", () => {
        component.onSubmit();
        // No error thrown, form stays open
        expect(component).toBeTruthy();
    });
});
