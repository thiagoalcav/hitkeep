import { ChangeDetectionStrategy, Component, inject } from '@angular/core';
import { RouterOutlet } from '@angular/router';
import { TranslocoPipe } from '@jsverse/transloco';
import { LayoutMobileHeader } from '@layout/layout-mobile-header';
import { LayoutOverlays } from '@layout/layout-overlays';
import { LayoutPageBar } from '@layout/layout-page-bar';
import { LayoutSidebar } from '@layout/layout-sidebar';
import { MainLayoutContextService } from '@layout/main-layout-context.service';
import { SidebarMenuService } from '@layout/sidebar-menu.service';

@Component({
    selector: 'app-main-layout',
    changeDetection: ChangeDetectionStrategy.OnPush,
    host: {
        '(document:keydown)': 'handleKeyboard($event)'
    },
    providers: [MainLayoutContextService, SidebarMenuService],
    imports: [RouterOutlet, LayoutSidebar, LayoutMobileHeader, LayoutPageBar, LayoutOverlays, TranslocoPipe],
    templateUrl: './main-layout.html',
    styleUrl: './main-layout.css'
})
export class MainLayout {
    protected readonly context = inject(MainLayoutContextService);

    handleKeyboard(event: KeyboardEvent) {
        if ((event.metaKey || event.ctrlKey) && event.key === 'k') {
            event.preventDefault();
            this.openSiteSettings();
        }
    }

    openSiteSettings(tab = '0') {
        this.context.openSiteSettings(tab);
    }

    constructor() {
        this.context.init();
    }
}
