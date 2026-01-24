import {ComponentFixture, TestBed} from '@angular/core/testing';
import {TrackingCode} from './tracking-code';

describe('TrackingCode', () => {
  let component: TrackingCode;
  let fixture: ComponentFixture<TrackingCode>;
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [TrackingCode]
    }).compileComponents();

    fixture = TestBed.createComponent(TrackingCode);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });
  it('should create', () => {
    expect(component).toBeTruthy();
  });
  it('should update snippet when toggles change', () => {
    const internals = component as TrackingCode & {
      snippetCode: () => string;
      collectDnt: { set: (value: boolean) => void };
      disableBeacon: { set: (value: boolean) => void };
    };
    const getSnippet = () => internals.snippetCode();

    expect(getSnippet()).toContain('hk.js');
    expect(getSnippet()).not.toContain('data-collect-dnt');
    expect(getSnippet()).not.toContain('data-disable-beacon');

    internals.collectDnt.set(true);
    fixture.detectChanges();
    expect(getSnippet()).toContain('data-collect-dnt="true"');

    internals.disableBeacon.set(true);
    fixture.detectChanges();
    expect(getSnippet()).toContain('hk.js');
    expect(getSnippet()).toContain('data-disable-beacon="true"');
  });
});
