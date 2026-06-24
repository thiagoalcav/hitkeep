import { ComponentFixture, TestBed } from '@angular/core/testing';
import { MainLayout } from '@layout/main-layout';
import { provideRouter } from '@angular/router';
import { By } from '@angular/platform-browser';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TranslocoTestingModule } from '@jsverse/transloco';
import { DashboardBootstrapService } from '@services/dashboard-bootstrap.service';
import { PermissionService } from '@services/permission.service';
import { ShareService } from '@services/share.service';
import { TeamService } from '@services/team.service';
import { SiteService } from '@features/sites/services/site.service';
import { LayoutPageBar } from './layout-page-bar';
import { LayoutSidebar } from './layout-sidebar';
import { MainLayoutContextService } from './main-layout-context.service';
import { MenuItem } from 'primeng/api';
import { vi } from 'vitest';

interface LayoutSidebarTestAccess {
    openSiteSettings(tab?: string): void;
    closeMobileDrawer(): void;
    mobileMenuItems(): MenuItem[];
    canCreateTeams(): boolean;
}

interface LayoutPageBarTestAccess {
    canCreateTeams(): boolean;
}

describe('MainLayout', () => {
    let component: MainLayout;
    let fixture: ComponentFixture<MainLayout>;
    let httpMock: HttpTestingController;
    let bootstrap: DashboardBootstrapService;
    let layoutContext: MainLayoutContextService;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [
                MainLayout,
                TranslocoTestingModule.forRoot({
                    langs: {
                        en: {
                            nav: {
                                utm: 'UTM',
                                utmBuilder: 'UTM Builder',
                                qrCodes: 'QR codes',
                                importExport: 'Import & Export',
                                importExportAria: 'Go to import and export',
                                expandItem: 'Expand {{item}}',
                                collapseItem: 'Collapse {{item}}'
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
            providers: [provideRouter([]), provideHttpClient(), provideHttpClientTesting()]
        }).compileComponents();

        fixture = TestBed.createComponent(MainLayout);
        component = fixture.componentInstance;
        httpMock = TestBed.inject(HttpTestingController);
        bootstrap = TestBed.inject(DashboardBootstrapService);
        layoutContext = fixture.debugElement.injector.get(MainLayoutContextService);
        seedLayoutState();
        fixture.detectChanges();
        fixture.detectChanges();
    });

    afterEach(() => {
        httpMock.verify();
        vi.restoreAllMocks();
    });

    it('should create', () => {
        expect(component).toBeTruthy();
    });

    it('A11Y: should have correct landmarks', () => {
        const aside = fixture.debugElement.query(By.css('aside'));
        const main = fixture.debugElement.query(By.css('main'));
        const nav = fixture.debugElement.query(By.css('nav'));

        expect(aside).toBeTruthy();
        expect(main).toBeTruthy();
        expect(nav).toBeTruthy();

        // Check labels
        expect(aside.attributes['aria-label']).toBeTruthy();
        expect(main.attributes['role']).toBe('main');
    });

    it('A11Y: buttons should have accessible labels', () => {
        const buttons = fixture.debugElement.queryAll(By.css('button'));
        const buttonsWithAria = buttons.filter((btn) => !!btn.attributes['aria-label']);
        expect(buttonsWithAria.length).toBeGreaterThan(0);
    });

    it('should always render team switcher', () => {
        const switchers = fixture.debugElement.queryAll(By.css('app-team-switcher'));
        expect(switchers.length).toBeGreaterThan(0);
    });

    it('should not render the legacy floating desktop account cluster', () => {
        const topbarCluster = fixture.nativeElement.querySelector('.layout-topbar__account-cluster');
        expect(topbarCluster).toBeNull();
    });

    it('should show administration section for team owner/admin role', () => {
        const adminLinks = Array.from(fixture.nativeElement.querySelectorAll('nav a')) as HTMLElement[];
        const hasTeamLink = adminLinks.some((link: HTMLElement) => link.getAttribute('href') === '/admin/team');
        expect(hasTeamLink).toBeTruthy();
    });

    it('should hide administration section for team member role', () => {
        const teamService = TestBed.inject(TeamService);
        teamService.teams.set([{ id: '00000000-0000-0000-0000-000000000001', name: 'Alpha Team', logo_url: '', role: 'member', created_at: '2026-01-01T00:00:00Z' }]);
        teamService.activeTeamId.set('00000000-0000-0000-0000-000000000001');
        fixture.detectChanges();
        const adminLinks = Array.from(fixture.nativeElement.querySelectorAll('nav a')) as HTMLElement[];
        const hasTeamLink = adminLinks.some((link: HTMLElement) => link.getAttribute('href') === '/admin/team');
        expect(hasTeamLink).toBeFalsy();
    });

    it('should show Search Console setup navigation for team owner/admin role', () => {
        const navLinks = Array.from(fixture.nativeElement.querySelectorAll('nav a')) as HTMLElement[];
        const hasSearchConsoleLink = navLinks.some((link: HTMLElement) => link.getAttribute('href') === '/integration/google-search-console');
        expect(hasSearchConsoleLink).toBeTruthy();
    });

    it('should hide Search Console setup navigation for team member role', () => {
        const teamService = TestBed.inject(TeamService);
        teamService.teams.set([{ id: '00000000-0000-0000-0000-000000000001', name: 'Alpha Team', logo_url: '', role: 'member', created_at: '2026-01-01T00:00:00Z' }]);
        teamService.activeTeamId.set('00000000-0000-0000-0000-000000000001');
        fixture.detectChanges();

        const navLinks = Array.from(fixture.nativeElement.querySelectorAll('nav a')) as HTMLElement[];
        const hasSearchConsoleLink = navLinks.some((link: HTMLElement) => link.getAttribute('href') === '/integration/google-search-console');
        expect(hasSearchConsoleLink).toBeFalsy();
    });

    it('should show Import & Export navigation to site viewers through the smart hub route', () => {
        const permissions = TestBed.inject(PermissionService);
        const siteService = TestBed.inject(SiteService);
        siteService.applySites([
            {
                id: '00000000-0000-0000-0000-0000000000aa',
                user_id: '00000000-0000-0000-0000-000000000001',
                domain: 'viewer.example.com',
                created_at: '2026-01-01T00:00:00Z'
            }
        ]);
        permissions.applyPermissions({
            instance_role: 'user',
            permissions: {
                '00000000-0000-0000-0000-0000000000aa': 'viewer'
            }
        });
        fixture.detectChanges();

        const navLinks = Array.from(fixture.nativeElement.querySelectorAll('nav a')) as HTMLAnchorElement[];
        const importExportLink = navLinks.find((link) => link.textContent?.includes('Import & Export'));

        expect(importExportLink).toBeTruthy();
        expect(importExportLink?.getAttribute('href')).toBe('/import-export');
        expect(importExportLink?.getAttribute('aria-label') ?? importExportLink?.closest('[role="treeitem"]')?.getAttribute('aria-label')).toBe('Import & Export');
    });

    it('should hide Import & Export navigation in share mode', () => {
        TestBed.inject(ShareService).setToken('share-token');
        fixture.detectChanges();

        const navLinks = Array.from(fixture.nativeElement.querySelectorAll('nav a')) as HTMLAnchorElement[];
        const importExportLink = navLinks.find((link) => link.getAttribute('href') === '/import-export');

        expect(importExportLink).toBeFalsy();
    });

    it('should keep collapsible sidebar parents navigable while the chevron expands children', () => {
        let utmLink = fixture.nativeElement.querySelector('aside a[href="/utm"]') as HTMLAnchorElement | null;
        let utmBuilderLink = fixture.nativeElement.querySelector('aside a[href="/utm/builder"]') as HTMLAnchorElement | null;
        let utmTreeItem = utmLink?.closest('[role="treeitem"]') as HTMLElement | null;
        let toggle = utmTreeItem?.querySelector('button.layout-sidebar-menu__toggle') as HTMLButtonElement | null;

        expect(utmLink).toBeTruthy();
        expect(utmBuilderLink).toBeNull();
        expect(utmTreeItem?.getAttribute('aria-expanded')).toBe('false');
        expect(toggle?.getAttribute('aria-label')).toBe('Expand UTM');

        toggle?.click();
        fixture.detectChanges();

        utmLink = fixture.nativeElement.querySelector('aside a[href="/utm"]') as HTMLAnchorElement | null;
        utmBuilderLink = fixture.nativeElement.querySelector('aside a[href="/utm/builder"]') as HTMLAnchorElement | null;
        utmTreeItem = utmLink?.closest('[role="treeitem"]') as HTMLElement | null;
        toggle = utmTreeItem?.querySelector('button.layout-sidebar-menu__toggle') as HTMLButtonElement | null;

        expect(utmLink).toBeTruthy();
        expect(utmBuilderLink).toBeTruthy();
        expect(utmTreeItem?.getAttribute('aria-expanded')).toBe('true');
        expect(toggle?.getAttribute('aria-label')).toBe('Collapse UTM');
        expect(utmTreeItem?.querySelector('ul.layout-sidebar-menu__list--nested')?.getAttribute('role')).toBe('group');
    });

    it('should hide create team actions in hosted cloud for non-owners', () => {
        TestBed.inject(PermissionService).applyPermissions({
            instance_role: 'user',
            permissions: {}
        });
        bootstrap.status.set({
            needs_setup: false,
            version: 'v2.0.0',
            cloud: { hosted: true, signup_enabled: false }
        });
        fixture.detectChanges();

        const switchers = fixture.debugElement.queryAll(By.css('app-team-switcher'));
        expect(switchers.length).toBeGreaterThan(0);
        for (const switcher of switchers) {
            expect(switcher.componentInstance.showAdd()).toBe(false);
        }
    });

    it('should show create team actions in hosted cloud for instance owners', () => {
        bootstrap.status.set({
            needs_setup: false,
            version: 'v2.0.0',
            cloud: { hosted: true, signup_enabled: false }
        });
        fixture.detectChanges();

        const switchers = fixture.debugElement.queryAll(By.css('app-team-switcher'));
        expect(switchers.length).toBeGreaterThan(0);
        for (const switcher of switchers) {
            expect(switcher.componentInstance.showAdd()).toBe(true);
        }
    });

    it('should show docs link in oss mode and hide support link', () => {
        const links = Array.from(fixture.nativeElement.querySelectorAll('a[href]')) as HTMLAnchorElement[];

        const docsLink = links.find((link) => link.href === 'https://hitkeep.com/guides/introduction/');
        const supportLink = links.find((link) => link.href === 'https://hitkeep.com/support/help/');

        expect(docsLink).toBeTruthy();
        expect(docsLink?.querySelector('.pi-external-link')).toBeTruthy();
        expect(supportLink).toBeFalsy();
    });

    it('should show support link in hosted cloud mode', () => {
        bootstrap.status.set({
            needs_setup: false,
            version: 'v2.0.0',
            cloud: {
                hosted: true,
                signup_enabled: false,
                support_url: 'https://hitkeep.com/support/help/'
            }
        });
        fixture.detectChanges();

        const links = Array.from(fixture.nativeElement.querySelectorAll('a[href]')) as HTMLAnchorElement[];
        const supportLink = links.find((link) => link.href === 'https://hitkeep.com/support/help/');

        expect(supportLink).toBeTruthy();
        expect(supportLink?.querySelector('.pi-headphones')).toBeTruthy();
        expect(supportLink?.querySelector('.pi-external-link')).toBeTruthy();
    });

    it('should allow team switch without confirmation when settings drawer is closed', () => {
        const confirmSpy = vi.spyOn(window, 'confirm');
        const result = layoutContext.beforeTeamSwitch();
        expect(result).toBe(true);
        expect(confirmSpy).not.toHaveBeenCalled();
    });

    it('should block team switch when settings drawer is open and user cancels', () => {
        const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
        layoutContext.isSiteSettingsVisible.set(true);
        const result = layoutContext.beforeTeamSwitch();
        expect(result).toBe(false);
        expect(confirmSpy).toHaveBeenCalled();
        expect(layoutContext.isSiteSettingsVisible()).toBe(true);
    });

    it('should close settings drawer when switch is confirmed', () => {
        const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
        layoutContext.isSiteSettingsVisible.set(true);
        const result = layoutContext.beforeTeamSwitch();
        expect(result).toBe(true);
        expect(confirmSpy).toHaveBeenCalled();
        expect(layoutContext.isSiteSettingsVisible()).toBe(false);
    });

    it('should open site settings from keyboard shortcut when an active site exists', () => {
        seedActiveSite();
        const event = new KeyboardEvent('keydown', { key: 'k', metaKey: true });
        const preventDefault = vi.spyOn(event, 'preventDefault');

        component.handleKeyboard(event);

        expect(preventDefault).toHaveBeenCalled();
        expect(layoutContext.isSiteSettingsVisible()).toBe(true);
        expect(layoutContext.siteSettingsTab()).toBe('0');
    });

    it('should handle the document keyboard shortcut binding and ctrl-key variant', () => {
        seedActiveSite();
        const event = new KeyboardEvent('keydown', { key: 'k', ctrlKey: true });
        const preventDefault = vi.spyOn(event, 'preventDefault');

        document.dispatchEvent(event);

        expect(preventDefault).toHaveBeenCalled();
        expect(layoutContext.isSiteSettingsVisible()).toBe(true);
    });

    it('should ignore unrelated keyboard shortcuts', () => {
        const event = new KeyboardEvent('keydown', { key: 'x', metaKey: true });
        const preventDefault = vi.spyOn(event, 'preventDefault');

        component.handleKeyboard(event);

        expect(preventDefault).not.toHaveBeenCalled();
        expect(layoutContext.isSiteSettingsVisible()).toBe(false);
    });

    it('should keep sidebar drawer actions inside the sidebar component', () => {
        seedActiveSite();
        const sidebar = fixture.debugElement.query(By.directive(LayoutSidebar)).componentInstance as LayoutSidebarTestAccess;
        layoutContext.isMobileDrawerOpen.set(true);

        sidebar.openSiteSettings();
        sidebar.closeMobileDrawer();
        const menuItems = sidebar.mobileMenuItems();
        const firstItem = menuItems[0]?.items?.[0];
        firstItem?.command?.({ originalEvent: new Event('click'), item: firstItem });

        expect(layoutContext.isSiteSettingsVisible()).toBe(true);
        expect(layoutContext.isMobileDrawerOpen()).toBe(false);
        expect(menuItems.length).toBeGreaterThan(0);
        expect(sidebar.canCreateTeams()).toBe(true);
    });

    it('should keep the mobile PrimeNG menu model stable between change detection passes', () => {
        const sidebar = fixture.debugElement.query(By.directive(LayoutSidebar)).componentInstance as LayoutSidebarTestAccess;

        const firstItems = sidebar.mobileMenuItems();
        fixture.detectChanges();
        const secondItems = sidebar.mobileMenuItems();

        expect(secondItems).toBe(firstItems);
    });

    it('should derive page-bar team creation affordance from cloud mode and instance owner role', () => {
        const pageBar = fixture.debugElement.query(By.directive(LayoutPageBar)).componentInstance as LayoutPageBarTestAccess;

        expect(pageBar.canCreateTeams()).toBe(true);

        bootstrap.status.set({
            needs_setup: false,
            version: 'v2.0.0',
            cloud: { hosted: true, signup_enabled: false }
        });

        expect(pageBar.canCreateTeams()).toBe(true);

        TestBed.inject(PermissionService).applyPermissions({
            instance_role: 'user',
            permissions: {}
        });

        expect(pageBar.canCreateTeams()).toBe(false);
    });

    function seedLayoutState() {
        const teamService = TestBed.inject(TeamService);
        const permissions = TestBed.inject(PermissionService);

        teamService.applyTeams({
            active_team_id: '00000000-0000-0000-0000-000000000001',
            teams: [
                {
                    id: '00000000-0000-0000-0000-000000000001',
                    name: 'Alpha Team',
                    logo_url: '',
                    role: 'owner',
                    created_at: '2026-01-01T00:00:00Z'
                },
                {
                    id: '00000000-0000-0000-0000-000000000002',

                    name: 'Beta Team',
                    logo_url: '',
                    role: 'admin',
                    created_at: '2026-01-02T00:00:00Z'
                }
            ]
        });

        bootstrap.status.set({
            needs_setup: false,
            version: 'v2.0.0',
            cloud: {
                hosted: false,
                signup_enabled: false
            }
        });

        permissions.applyPermissions({
            instance_role: 'owner',
            permissions: {}
        });
    }

    function seedActiveSite() {
        TestBed.inject(SiteService).applySites([
            {
                id: '00000000-0000-0000-0000-0000000000bb',
                user_id: '00000000-0000-0000-0000-000000000001',
                domain: 'active.example.com',
                created_at: '2026-01-01T00:00:00Z'
            }
        ]);
    }
});
