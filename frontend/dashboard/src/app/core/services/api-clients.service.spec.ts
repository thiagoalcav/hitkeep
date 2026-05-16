import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';

import { APIClientsService } from './api-clients.service';

describe('APIClientsService', () => {
    let service: APIClientsService;
    let httpMock: HttpTestingController;

    beforeEach(() => {
        TestBed.configureTestingModule({
            providers: [provideHttpClient(), provideHttpClientTesting()]
        });
        service = TestBed.inject(APIClientsService);
        httpMock = TestBed.inject(HttpTestingController);
    });

    afterEach(() => {
        httpMock.verify();
    });

    it('rotates personal clients through the personal endpoint', () => {
        service.rotateClient('client-1').subscribe();

        const req = httpMock.expectOne('/api/user/api-clients/client-1/rotate');
        expect(req.request.method).toBe('POST');
        expect(req.request.body).toEqual({});
        req.flush({ client: null, token: 'new-token' });
    });

    it('rotates team clients through the team endpoint', () => {
        service.rotateClient('client-1', 'team-1').subscribe();

        const req = httpMock.expectOne('/api/user/teams/team-1/api-clients/client-1/rotate');
        expect(req.request.method).toBe('POST');
        req.flush({ client: null, token: 'new-token' });
    });
});
