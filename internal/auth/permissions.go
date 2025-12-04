package auth

type InstanceRole string

const (
	InstanceOwner InstanceRole = "owner"
	InstanceAdmin InstanceRole = "admin"
	InstanceUser  InstanceRole = "user"
)

type SiteRole string

const (
	SiteOwner  SiteRole = "owner"
	SiteAdmin  SiteRole = "admin"
	SiteEditor SiteRole = "editor"
	SiteViewer SiteRole = "viewer"
)

// Permission matrix
type Permission string

const (
	// Instance permissions
	PermInstanceManageUsers    Permission = "instance.manage_users"
	PermInstanceViewAllSites   Permission = "instance.view_all_sites"
	PermInstanceManageSettings Permission = "instance.manage_settings"

	// Site permissions
	PermSiteView        Permission = "site.view"
	PermSiteManageData  Permission = "site.manage_data"
	PermSiteManageGoals Permission = "site.manage_goals"
	PermSiteManageTeam  Permission = "site.manage_team"
	PermSiteDelete      Permission = "site.delete"
)

// Role to permissions mapping
var instancePermissions = map[InstanceRole][]Permission{
	InstanceOwner: {
		PermInstanceManageUsers,
		PermInstanceViewAllSites,
		PermInstanceManageSettings,
	},
	InstanceAdmin: {
		PermInstanceViewAllSites,
	},
	InstanceUser: {},
}

var sitePermissions = map[SiteRole][]Permission{
	SiteOwner: {
		PermSiteView,
		PermSiteManageData,
		PermSiteManageGoals,
		PermSiteManageTeam,
		PermSiteDelete,
	},
	SiteAdmin: {
		PermSiteView,
		PermSiteManageData,
		PermSiteManageGoals,
		PermSiteManageTeam,
	},
	SiteEditor: {
		PermSiteView,
		PermSiteManageGoals,
	},
	SiteViewer: {
		PermSiteView,
	},
}

func (r InstanceRole) HasPermission(perm Permission) bool {
	perms, ok := instancePermissions[r]
	if !ok {
		return false
	}
	for _, p := range perms {
		if p == perm {
			return true
		}
	}
	return false
}

func (r SiteRole) HasPermission(perm Permission) bool {
	perms, ok := sitePermissions[r]
	if !ok {
		return false
	}
	for _, p := range perms {
		if p == perm {
			return true
		}
	}
	return false
}
