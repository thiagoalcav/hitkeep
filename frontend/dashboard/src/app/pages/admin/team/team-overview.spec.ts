import { ComponentFixture, TestBed } from "@angular/core/testing";
import { signal } from "@angular/core";
import { NoopAnimationsModule } from "@angular/platform-browser/animations";
import { TranslocoTestingModule } from "@jsverse/transloco";
import { TeamOverviewPage } from "./team-overview";
import { TeamService } from "@services/team.service";
import { Team } from "@models/analytics.types";

describe("TeamOverviewPage", () => {
    let fixture: ComponentFixture<TeamOverviewPage>;

    const activeTeam = signal<Team | null>({
        id: "00000000-0000-0000-0000-000000000001",
        name: "Acme Analytics",
        logo_url: "",
        role: "owner",
        created_at: "2026-03-01T00:00:00Z",
        usage: {
            current_sites: 3,
            current_members: 5,
            current_pending_invites: 2,
            current_monthly_events: 1450
        },
        entitlements: {
            max_sites_per_team: 10,
            max_team_members: 8,
            max_monthly_events: 2000,
            max_retention_days: 365,
            allow_sso: true,
            allow_custom_branding: true
        }
    });

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [
                TeamOverviewPage,
                NoopAnimationsModule,
                TranslocoTestingModule.forRoot({
                    langs: {
                        en: {
                            admin: {
                                team: {
                                    overview: {
                                        title: "Team",
                                        memberCount: "Members",
                                        usage: {
                                            title: "Usage and limits",
                                            description: "Track plan usage before it becomes a limit.",
                                            currentUsage: "{{count}} used",
                                            unlimited: "Unlimited",
                                            unlimitedFootnote: "Unlimited on the current plan",
                                            limitValue: "Limit {{count}}",
                                            progressLabel: "{{count}}% of limit",
                                            pendingInviteOne: "1 pending invite",
                                            pendingInviteMany: "{{count}} pending invites",
                                            metrics: {
                                                monthlyEvents: "Monthly events",
                                                sites: "Sites",
                                                members: "Members"
                                            }
                                        }
                                    }
                                }
                            },
                            common: { columns: { role: "Role" } },
                            teams: { roles: { owner: "Owner" } }
                        }
                    },
                    translocoConfig: {
                        availableLangs: ["en"],
                        defaultLang: "en"
                    }
                })
            ],
            providers: [
                {
                    provide: TeamService,
                    useValue: {
                        activeTeam
                    }
                }
            ]
        }).compileComponents();

        fixture = TestBed.createComponent(TeamOverviewPage);
        fixture.detectChanges();
    });

    it("renders usage cards for the active team", () => {
        const text = fixture.nativeElement.textContent;
        expect(text).toContain("Usage and limits");
        expect(text).toContain("Monthly events");
        expect(text).toContain("Sites");
        expect(text).toContain("Members");
        expect(text).toContain("2 pending invites");
        expect(text).toContain("1450");
    });
});
