package auth

import "testing"

func TestManagementAccountAuthority(t *testing.T) {
	for _, test := range []struct {
		name    string
		actor   Role
		target  string
		kind    string
		allowed bool
	}{
		{"system manages user", RoleSysAdmin, "USER", "profile", true},
		{"system manages administrator", RoleSysAdmin, "ADMIN", "status", true},
		{"administrator manages user", RoleAdmin, "USER", "profile", true},
		{"administrator target requires system role", RoleAdmin, "ADMIN", "profile", false},
		{"system target requires system role", RoleAdmin, "SYS_ADMIN", "sessions/revoke", false},
		{"promotion requires system role", RoleAdmin, "USER", "role", false},
		{"system promotes user", RoleSysAdmin, "USER", "role", true},
		{"system changes another system account role", RoleSysAdmin, "SYS_ADMIN", "role", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			actor := User{Role: test.actor, Permissions: []Permission{PermissionUserManage}}
			target := dbUser{Role: test.target}
			err := authorizeManagedMutation(actor, target, managedMutation{Kind: test.kind, Role: RoleAdmin})
			if (err == nil) != test.allowed {
				t.Fatalf("authorization=%v allowed=%v", err, test.allowed)
			}
		})
	}
}
