import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';

import { AdminSystemService } from './admin-system.service';

describe('AdminSystemService', () => {
    let service: AdminSystemService;
    let httpMock: HttpTestingController;

    beforeEach(() => {
        TestBed.configureTestingModule({
            providers: [provideHttpClient(), provideHttpClientTesting()]
        });

        service = TestBed.inject(AdminSystemService);
        httpMock = TestBed.inject(HttpTestingController);
    });

    afterEach(() => {
        httpMock.verify();
    });

    it('keeps pagination only on audit list requests', () => {
        service.listAudit({ action: 'mail.test', target_type: 'mail', limit: 25, offset: 50 }).subscribe();

        const req = httpMock.expectOne((request) => request.url === '/api/admin/system/audit');
        expect(req.request.params.get('action')).toBe('mail.test');
        expect(req.request.params.get('target_type')).toBe('mail');
        expect(req.request.params.get('limit')).toBe('25');
        expect(req.request.params.get('offset')).toBe('50');
        req.flush({ entries: [], total: 0, limit: 25, offset: 50, has_more: false });
    });

    it('exports audit filters with the backend export cap instead of the visible page size', () => {
        service.exportAudit({ action: 'spam_filter.refresh', target_type: 'spam_filter', limit: 25, offset: 50 }).subscribe();

        const req = httpMock.expectOne((request) => request.url === '/api/admin/system/audit/export');
        expect(req.request.params.get('action')).toBe('spam_filter.refresh');
        expect(req.request.params.get('target_type')).toBe('spam_filter');
        expect(req.request.params.get('format')).toBe('json');
        expect(req.request.params.get('limit')).toBe('50000');
        expect(req.request.params.has('offset')).toBe(false);
        req.flush(new Blob(['[]'], { type: 'application/json' }));
    });
});
