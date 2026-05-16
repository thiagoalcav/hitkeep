import { ComponentFixture, TestBed } from '@angular/core/testing';
import { TranslocoTestingModule } from '@jsverse/transloco';
import { of } from 'rxjs';
import { vi } from 'vitest';

import { GoalManager } from './goal-manager';
import { AnalyticsService } from '@services/analytics.service';
import { Goal } from '@models/analytics.types';

describe('GoalManager', () => {
    let fixture: ComponentFixture<GoalManager>;

    const goal: Goal = {
        id: 'goal-1',
        site_id: 'site-1',
        name: 'Signup',
        type: 'path',
        value: '/signup',
        created_at: '2026-01-01T00:00:00Z'
    };

    const analyticsService = {
        getGoals: vi.fn(() => of([goal])),
        createGoal: vi.fn(() => of(goal)),
        deleteGoal: vi.fn(() => of(void 0))
    };

    beforeEach(async () => {
        vi.clearAllMocks();

        await TestBed.configureTestingModule({
            imports: [
                GoalManager,
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
                                    name: 'Name',
                                    type: 'Type',
                                    value: 'Value'
                                },
                                searchPlaceholder: 'Search'
                            },
                            goals: {
                                manager: {
                                    dialogTitle: 'Manage goals',
                                    dialogDescription: 'Define goals',
                                    empty: 'No goals',
                                    newAction: 'New goal',
                                    addTitle: 'Add new goal',
                                    editTitle: 'Edit goal',
                                    editTooltip: 'Edit goal',
                                    deleteTooltip: 'Delete goal',
                                    confirmDelete: 'Delete goal {{name}}?',
                                    namePlaceholder: 'Goal name',
                                    typePagePath: 'Path',
                                    typeCustomEvent: 'Event',
                                    urlPathLabel: 'Path',
                                    eventNameLabel: 'Event',
                                    urlPathPlaceholder: '/thank-you',
                                    eventNamePlaceholder: 'signup_completed',
                                    urlPathHelp: 'Triggers on path.',
                                    eventNameHelpPrefix: 'Triggers when you call',
                                    eventNameHelpSuffix: '.',
                                    createAction: 'Create goal',
                                    saveAction: 'Save changes',
                                    messages: {
                                        createSuccess: 'Goal created.',
                                        createError: 'Goal could not be created.',
                                        updateSuccess: 'Goal updated.',
                                        updateError: 'Goal could not be updated.',
                                        deleteSuccess: 'Goal deleted.',
                                        deleteError: 'Goal could not be deleted.'
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

        fixture = TestBed.createComponent(GoalManager);
        fixture.componentRef.setInput('siteId', 'site-1');
        fixture.componentRef.setInput('visible', true);
        fixture.detectChanges();
    });

    afterEach(() => {
        document.querySelectorAll('.p-dialog-mask, .p-dialog, .p-confirm-dialog, .table-row-actions-menu').forEach((element) => element.remove());
    });

    it('shows the goal table first and opens create in the shared CRUD dialog', async () => {
        await fixture.whenStable();

        expect(document.body.querySelector('#g-name')).toBeNull();
        expect(document.body.textContent).toContain('Signup');

        clickBodyButton('New goal');
        fixture.detectChanges();
        await fixture.whenStable();

        expect(document.body.textContent).toContain('Add new goal');
        expect(document.body.querySelector('#g-name')).not.toBeNull();
    });

    it('opens edit from the shared row-action menu', async () => {
        await fixture.whenStable();

        clickBodyButton('More actions');
        fixture.detectChanges();
        await fixture.whenStable();

        clickBodyMenuItem('Edit goal');
        fixture.detectChanges();
        await fixture.whenStable();

        expect(document.body.textContent).toContain('Edit goal');
        expect((document.body.querySelector('#g-name') as HTMLInputElement | null)?.value).toBe('Signup');
    });

    it('keeps the manager close action after closing a nested editor dialog', async () => {
        await fixture.whenStable();

        clickBodyButton('More actions');
        fixture.detectChanges();
        await fixture.whenStable();

        clickBodyMenuItem('Edit goal');
        fixture.detectChanges();
        await fixture.whenStable();

        clickBodyButton('Cancel');
        fixture.detectChanges();
        await fixture.whenStable();

        expect(visibleDialogText()).toContain('Manage goals');
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
