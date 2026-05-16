import { ComponentFixture, TestBed } from '@angular/core/testing';
import { TranslocoTestingModule } from '@jsverse/transloco';
import { of } from 'rxjs';
import { vi } from 'vitest';

import { FunnelManager } from './funnel-manager';
import { AnalyticsService } from '@services/analytics.service';
import { Funnel } from '@models/analytics.types';

describe('FunnelManager', () => {
    let fixture: ComponentFixture<FunnelManager>;

    const funnel: Funnel = {
        id: 'funnel-1',
        site_id: 'site-1',
        name: 'Checkout',
        steps: [
            { type: 'path', value: '/cart' },
            { type: 'event', value: 'purchase_completed' }
        ],
        created_at: '2026-01-01T00:00:00Z'
    };

    const analyticsService = {
        getFunnels: vi.fn(() => of([funnel])),
        createFunnel: vi.fn(() => of(funnel)),
        deleteFunnel: vi.fn(() => of(void 0))
    };

    beforeEach(async () => {
        vi.clearAllMocks();

        await TestBed.configureTestingModule({
            imports: [
                FunnelManager,
                TranslocoTestingModule.forRoot({
                    langs: {
                        en: {
                            common: {
                                actions: {
                                    cancel: 'Cancel',
                                    close: 'Close',
                                    more: 'More actions'
                                },
                                columns: {
                                    actions: 'Actions',
                                    name: 'Name'
                                },
                                searchPlaceholder: 'Search'
                            },
                            funnels: {
                                manager: {
                                    dialogTitle: 'Manage funnels',
                                    dialogDescription: 'Track journeys',
                                    stepsLabel: 'Steps',
                                    viewStats: 'View stats',
                                    empty: 'No funnels',
                                    newAction: 'New funnel',
                                    createTitle: 'Create new funnel',
                                    editTitle: 'Edit funnel',
                                    editTooltip: 'Edit funnel',
                                    deleteTooltip: 'Delete funnel',
                                    confirmDelete: 'Delete funnel {{name}}?',
                                    namePlaceholder: 'Funnel name',
                                    stepPathPlaceholder: '/path',
                                    stepEventPlaceholder: 'event_name',
                                    addStep: 'Add step',
                                    createAction: 'Create funnel',
                                    saveAction: 'Save changes',
                                    typePagePath: 'Path',
                                    typeCustomEvent: 'Event',
                                    messages: {
                                        createSuccess: 'Funnel created.',
                                        createError: 'Funnel could not be created.',
                                        updateSuccess: 'Funnel updated.',
                                        updateError: 'Funnel could not be updated.',
                                        deleteSuccess: 'Funnel deleted.',
                                        deleteError: 'Funnel could not be deleted.'
                                    }
                                }
                            }
                        }
                    },
                    translocoConfig: {
                        availableLangs: ['en'],
                        defaultLang: 'en'
                    },
                    preloadLangs: true
                })
            ],
            providers: [{ provide: AnalyticsService, useValue: analyticsService }]
        }).compileComponents();

        fixture = TestBed.createComponent(FunnelManager);
        fixture.componentRef.setInput('siteId', 'site-1');
        fixture.componentRef.setInput('visible', true);
        fixture.detectChanges();
    });

    afterEach(() => {
        document.querySelectorAll('.p-dialog-mask, .p-dialog, .p-confirm-dialog, .table-row-actions-menu').forEach((element) => element.remove());
    });

    it('shows the funnel table first and opens create in the shared CRUD dialog', async () => {
        await fixture.whenStable();

        expect(document.body.querySelector('#f-name')).toBeNull();
        expect(document.body.textContent).toContain('Checkout');

        clickBodyButton('New funnel');
        fixture.detectChanges();
        await fixture.whenStable();

        expect(document.body.textContent).toContain('Create new funnel');
        expect(document.body.querySelector('#f-name')).not.toBeNull();
    });

    it('opens edit from the shared row-action menu', async () => {
        await fixture.whenStable();

        clickBodyButton('More actions');
        fixture.detectChanges();
        await fixture.whenStable();

        clickBodyMenuItem('Edit funnel');
        fixture.detectChanges();
        await fixture.whenStable();

        expect(document.body.textContent).toContain('Edit funnel');
        expect((document.body.querySelector('#f-name') as HTMLInputElement | null)?.value).toBe('Checkout');
    });

    it('keeps the manager close action after closing a nested editor dialog', async () => {
        await fixture.whenStable();

        clickBodyButton('More actions');
        fixture.detectChanges();
        await fixture.whenStable();

        clickBodyMenuItem('Edit funnel');
        fixture.detectChanges();
        await fixture.whenStable();

        clickBodyButton('Cancel');
        fixture.detectChanges();
        await fixture.whenStable();

        expect(visibleDialogText()).toContain('Manage funnels');
        expect(visibleDialogFooterButtons().map((button) => button.textContent?.trim())).toContain('Close');
    });

    function clickBodyButton(label: string): void {
        const button = Array.from(document.body.querySelectorAll('button') as NodeListOf<HTMLButtonElement>).find((entry) => entry.textContent?.includes(label) || entry.getAttribute('aria-label') === label);
        button?.click();
    }

    function clickBodyMenuItem(label: string): void {
        const item = Array.from(document.body.querySelectorAll('.table-row-actions-menu a') as NodeListOf<HTMLAnchorElement>).find((entry) => entry.textContent?.includes(label));
        item?.click();
    }

    function visibleDialogText(): string {
        return visibleDialogs()
            .map((dialog) => dialog.textContent ?? '')
            .join(' ');
    }

    function visibleDialogFooterButtons(): HTMLButtonElement[] {
        return visibleDialogs().flatMap((dialog) => Array.from(dialog.querySelectorAll('.dialog-shell-footer button') as NodeListOf<HTMLButtonElement>));
    }

    function visibleDialogs(): HTMLElement[] {
        return Array.from(document.body.querySelectorAll('.p-dialog') as NodeListOf<HTMLElement>).filter((dialog) => {
            const rect = dialog.getBoundingClientRect();
            return rect.width > 0 && rect.height > 0;
        });
    }
});
