import { ChangeDetectionStrategy, Component, inject, input } from '@angular/core';
import { AvatarModule } from 'primeng/avatar';
import { MenuModule } from 'primeng/menu';
import { PreferencesService } from '@services/preferences.service';
import { UserMenuService } from '@services/user-menu.service';
import { UserProfileService } from '@services/user-profile.service';

@Component({
    selector: 'app-user-controls',
    standalone: true,
    imports: [AvatarModule, MenuModule],
    changeDetection: ChangeDetectionStrategy.OnPush,
    template: `
        <div class="flex items-center gap-2">
            <button
                type="button"
                (click)="prefs.toggleTheme()"
                class="cursor-pointer p-2 rounded-full hover:bg-surface-100 dark:hover:bg-surface-800 text-muted-color focus:outline-none focus:ring-2 focus:ring-primary-500"
                [attr.aria-label]="prefs.isDarkMode() ? 'Switch to Light Mode' : 'Switch to Dark Mode'"
            >
                <i class="pi" [class]="prefs.isDarkMode() ? 'pi-moon' : 'pi-sun'" aria-hidden="true"></i>
            </button>

            @if (showMenu()) {
                <button
                    type="button"
                    (click)="profileMenu.toggle($event)"
                    class="cursor-pointer flex items-center gap-2 px-2 py-1.5 rounded-full hover:bg-surface-100 dark:hover:bg-surface-800 focus:outline-none focus:ring-2 focus:ring-primary-500"
                    aria-haspopup="true"
                    aria-label="Open user menu"
                >
                    @if (profile.avatarUrl()) {
                        <p-avatar [image]="profile.avatarUrl()" shape="circle" styleClass="bg-surface-200 dark:bg-surface-700" aria-hidden="true" />
                    } @else {
                        <p-avatar icon="pi pi-user" shape="circle" styleClass="bg-surface-200 dark:bg-surface-700" aria-hidden="true" />
                    }
                    <span class="hidden sm:inline text-sm font-medium text-[var(--p-text-color)]">{{ profile.displayName() }}</span>
                    <i class="pi pi-chevron-down text-xs text-muted-color" aria-hidden="true"></i>
                </button>
                <p-menu #profileMenu [model]="userMenu.menuItems()" [popup]="true" appendTo="body" />
            }
        </div>
    `
})
export class UserControls {
    showMenu = input<boolean>(true);

    protected prefs = inject(PreferencesService);
    protected userMenu = inject(UserMenuService);
    protected profile = inject(UserProfileService);
}
