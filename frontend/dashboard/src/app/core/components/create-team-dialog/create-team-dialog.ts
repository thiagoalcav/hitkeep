import { ChangeDetectionStrategy, Component, inject, model, signal } from "@angular/core";
import { HttpErrorResponse } from "@angular/common/http";
import { ReactiveFormsModule, FormControl, Validators } from "@angular/forms";
import { compatForm } from "@angular/forms/signals/compat";
import { TranslocoPipe } from "@jsverse/transloco";
import { DialogModule } from "primeng/dialog";
import { ButtonModule } from "primeng/button";
import { InputTextModule } from "primeng/inputtext";
import { SiteService } from "@features/sites/services/site.service";
import { TeamService } from "@services/team.service";
import { PermissionService } from "@services/permission.service";

@Component({
    selector: "app-create-team-dialog",
    changeDetection: ChangeDetectionStrategy.OnPush,
    imports: [ReactiveFormsModule, DialogModule, ButtonModule, InputTextModule, TranslocoPipe],
    template: `
        <p-dialog [header]="'teams.createDialog.title' | transloco" [(visible)]="visible" [modal]="true" [style]="{ width: '450px', maxWidth: '90vw' }" (onHide)="resetForm()">
            <form (submit)="onSubmit($event)" class="flex flex-col gap-6 pt-2" novalidate>
                <div class="flex flex-col gap-2">
                    <label for="team-name" class="font-semibold text-sm text-[var(--p-text-color)]">{{ "teams.createDialog.nameLabel" | transloco }}</label>
                    <input pInputText id="team-name" [formControl]="form.name().control()" [placeholder]="'teams.createDialog.namePlaceholder' | transloco" class="w-full" [class.ng-invalid]="isInvalid()" [class.ng-dirty]="form.name().dirty()" />
                    @if (isInvalid() && form.name().control().hasError("required")) {
                        <small class="text-red-500">{{ "teams.createDialog.nameRequired" | transloco }}</small>
                    }
                    @if (createError()) {
                        <small class="text-red-500">{{ createError()! | transloco }}</small>
                    }
                </div>
            </form>

            <ng-template pTemplate="footer">
                <p-button [label]="'common.actions.cancel' | transloco" (onClick)="visible.set(false)" styleClass="p-button-text" />
                <p-button [label]="'teams.createDialog.createAction' | transloco" (onClick)="onSubmit()" [loading]="isSubmitting()" [disabled]="isSubmitting() || form().invalid()" />
            </ng-template>
        </p-dialog>
    `
})
export class CreateTeamDialog {
    visible = model<boolean>(false);

    private teamService = inject(TeamService);
    private siteService = inject(SiteService);
    private perms = inject(PermissionService);

    protected isSubmitting = signal(false);
    protected createError = signal<string | null>(null);

    private readonly formModel = signal({
        name: new FormControl("", { nonNullable: true, validators: [Validators.required, Validators.maxLength(120)] })
    });
    protected readonly form = compatForm(this.formModel);

    protected isInvalid() {
        return this.form.name().invalid() && (this.form.name().dirty() || this.form.name().touched());
    }

    resetForm() {
        this.form.name().control().reset("");
        this.createError.set(null);
        this.isSubmitting.set(false);
    }

    onSubmit(event?: Event) {
        event?.preventDefault();
        if (this.form().invalid()) {
            this.form.name().markAsTouched();
            return;
        }

        const name = this.form.name().value().trim();
        if (!name) {
            return;
        }

        this.isSubmitting.set(true);
        this.createError.set(null);

        this.teamService.createTeam({ name, logo_url: "" }).subscribe({
            next: () => {
                this.siteService.sites.set([]);
                this.siteService.activeSite.set(null);
                this.teamService.loadTeams().subscribe({
                    next: () => {
                        this.siteService.loadSites();
                        this.perms.loadPermissions().subscribe({ error: () => undefined });
                        this.visible.set(false);
                    },
                    error: () => {
                        this.siteService.loadSites();
                        this.perms.loadPermissions().subscribe({ error: () => undefined });
                        this.visible.set(false);
                    }
                });
            },
            error: (err: HttpErrorResponse) => {
                this.createError.set(err.status === 403 ? "teams.createDialog.errors.limitReached" : "teams.createDialog.errors.createFailed");
                this.isSubmitting.set(false);
            }
        });
    }
}
