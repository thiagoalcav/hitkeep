import { inject } from '@angular/core';
import { CanActivateFn, Router, UrlTree } from '@angular/router';
import { TeamService } from '@services/team.service';
import { map, of, switchMap } from 'rxjs';

export const teamAdminGuard: CanActivateFn = () => {
    const teamService = inject(TeamService);
    const router = inject(Router);

    const checkTeamAdmin = (): boolean | UrlTree => {
        const role = teamService.activeTeam()?.role;
        if (role === 'owner' || role === 'admin') {
            return true;
        }
        return router.createUrlTree(['/dashboard']);
    };

    if (teamService.teams().length > 0) {
        return checkTeamAdmin();
    }

    return teamService.loadTeams().pipe(
        map(() => checkTeamAdmin()),
        switchMap((result) => of(result))
    );
};
