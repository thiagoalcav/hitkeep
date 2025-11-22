import { TestBed } from '@angular/core/testing';
import { PreferencesService } from './preferences.service';

describe('PreferencesService', () => {
  let service: PreferencesService;

  beforeEach(() => {
    TestBed.configureTestingModule({
        providers: [PreferencesService]
    });
    service = TestBed.inject(PreferencesService);

    // Mock LocalStorage
    const store: Record<string, string> = {};
    spyOn(localStorage, 'getItem').and.callFake((key) => store[key] || null);
    spyOn(localStorage, 'setItem').and.callFake((key, value) => store[key] = value + '');
  });

  it('should be created', () => {
    expect(service).toBeTruthy();
  });

  it('should toggle theme signal', () => {
    const initial = service.isDarkMode();
    service.toggleTheme();
    expect(service.isDarkMode()).not.toBe(initial);
  });
});
