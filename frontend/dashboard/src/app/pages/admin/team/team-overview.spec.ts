import { ComponentFixture, TestBed } from "@angular/core/testing";
import { signal } from "@angular/core";
import { NoopAnimationsModule } from "@angular/platform-browser/animations";
import { TranslocoTestingModule } from "@jsverse/transloco";
import { TeamOverviewPage } from "./team-overview";
import { TeamService } from "@services/team.service";
import { AnalyticsService } from "@services/analytics.service";
import { Team } from "@models/analytics.types";
import { of } from "rxjs";

describe("TeamOverviewPage", () => {
    let fixture: ComponentFixture<TeamOverviewPage>;
    let systemStatusResponse: {
        needs_setup: boolean;
        version: string;
        cloud?: {
            hosted: boolean;
            signup_enabled: boolean;
            jurisdiction?: string;
            region?: string;
            upgrade_url?: string;
            support_url?: string;
        };
    };

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
        },
        plan: {
            code: "free",
            name: "Free",
            upgrade_url: "https://hitkeep.com/cloud/upgrade",
            support_url: "https://hitkeep.com/cloud/support"
        }
    });

    beforeEach(async () => {
        systemStatusResponse = {
            needs_setup: false,
            version: "v2.0.0",
            cloud: {
                hosted: true,
                signup_enabled: false,
                jurisdiction: "EU",
                region: "eu-central-1",
                upgrade_url: "https://hitkeep.com/cloud/upgrade",
                support_url: "https://hitkeep.com/cloud/support"
            }
        };

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
                                        },
                                        cloud: {
                                            title: "Cloud plan",
                                            managed: "Managed cloud",
                                            description: "Run this team in the hosted EU or US cell with hard limits, backups, and upgrade paths.",
                                            jurisdiction: "Jurisdiction",
                                            region: "Region",
                                            retention: "Retention",
                                            retentionDays: "{{count}} days",
                                            unlimitedRetention: "Unlimited retention",
                                            upgradeAction: "Upgrade plan",
                                            supportAction: "Contact support"
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
                },
                {
                    provide: AnalyticsService,
                    useValue: {
                        getSystemStatus: () => of(systemStatusResponse)
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

    it("renders cloud upgrade details when managed metadata is present", () => {
        const text = fixture.nativeElement.textContent;
        expect(text).toContain("Cloud plan");
        expect(text).toContain("Managed cloud");
        expect(text).toContain("EU");
        expect(text).toContain("365 days");
        expect(text).toContain("Upgrade plan");
        expect(text).toContain("Contact support");
    });

    it("hides usage limits and cloud plan details for OSS instances", () => {
        fixture.destroy();
        systemStatusResponse = {
            needs_setup: false,
            version: "v2.0.0"
        };

        fixture = TestBed.createComponent(TeamOverviewPage);
        fixture.detectChanges();

        const text = fixture.nativeElement.textContent;
        expect(text).not.toContain("Usage and limits");
        expect(text).not.toContain("Cloud plan");
        expect(text).toContain("Acme Analytics");
        expect(text).toContain("Members");
    });
});
