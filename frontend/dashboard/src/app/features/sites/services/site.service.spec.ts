import { TestBed } from "@angular/core/testing";
import { HttpTestingController, provideHttpClientTesting } from "@angular/common/http/testing";
import { provideHttpClient } from "@angular/common/http";
import { SiteService } from "@features/sites/services/site.service";

describe("SiteService", () => {
    let service: SiteService;
    let httpMock: HttpTestingController;

    beforeEach(() => {
        TestBed.configureTestingModule({
            providers: [SiteService, provideHttpClient(), provideHttpClientTesting()]
        });
        service = TestBed.inject(SiteService);
        httpMock = TestBed.inject(HttpTestingController);
    });

    afterEach(() => {
        httpMock.verify();
    });

    it("should be created", () => {
        expect(service).toBeTruthy();
    });

    it("should clear active site when the active team has no sites", () => {
        service.activeSite.set({
            id: "site-1",
            user_id: "user-1",
            domain: "example.com",
            created_at: "2026-01-01T00:00:00Z"
        });

        service.loadSites();

        const req = httpMock.expectOne("/api/sites");
        expect(req.request.method).toBe("GET");
        req.flush([]);

        expect(service.activeSite()).toBeNull();
        expect(service.sites()).toEqual([]);
    });
});
