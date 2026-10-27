package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/fdsouvenir/m365cli/internal/config"
	"github.com/fdsouvenir/m365cli/internal/secrets"
)

// ReadGrant uses .default only to inspect the existing delegated Graph grant.
// Its access token and scope string never replace the command authorization cache.
func (m *Manager) ReadGrant(ctx context.Context, p config.ProfileRecord) config.GrantRecord {
	checked := m.nowUTC()
	grant := config.GrantRecord{Status: "unknown", CheckedAt: &checked}
	fail := func(reason string) config.GrantRecord { grant.Scopes = nil; grant.Reason = reason; return grant }
	unlock, err := lockProfile(ctx, p.Name)
	if err != nil {
		return fail("Could not lock the token cache for grant inspection.")
	}
	defer unlock()
	cache, err := m.loadToken(p.Name)
	if err != nil {
		return fail("No usable token cache for grant inspection.")
	}
	if err := m.validateCachedDelegatedIdentity(p.Name, cache, p.AccountID, p.TenantID); err != nil {
		return fail("Token identity does not match the selected profile.")
	}
	if cache.RefreshToken == "" {
		return fail("No refresh token is available for grant inspection.")
	}
	req := url.Values{"grant_type": {"refresh_token"}, "client_id": {p.ClientID}, "refresh_token": {cache.RefreshToken}, "scope": {graphDefaultScope()}}
	if key, err := delegatedSecretKey(p.Name); err == nil {
		if secret, err := secrets.GetSecret(key); err == nil && len(secret) > 0 {
			req.Set("client_secret", string(secret))
			secureZero(secret)
		}
	}
	body, status, err := m.doFormWithStatus(ctx, endpoint(p.Authority, "/oauth2/v2.0/token"), req)
	if err != nil {
		return fail("Microsoft grant inspection could not be completed.")
	}
	defer secureZero(body)
	if status != http.StatusOK {
		return fail(fmt.Sprintf("Microsoft did not return the current grant (HTTP %d).", status))
	}
	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return fail("Microsoft returned an unreadable grant response.")
	}
	if tr.IDToken != "" {
		cache.IDToken = tr.IDToken
	}
	if err := m.validateCachedDelegatedIdentity(p.Name, cache, p.AccountID, p.TenantID); err != nil {
		return fail("Grant response identity does not match the selected profile.")
	}
	if tr.RefreshToken != "" {
		cache.RefreshToken = tr.RefreshToken
	}
	// Preserve refresh-token rotation, but discard the grant-inspection access token.
	if err := m.saveToken(p.Name, cache); err != nil {
		return fail("Could not save the rotated refresh token after grant inspection.")
	}
	if tr.AccessToken == "" {
		return fail("Microsoft returned no token with the grant scope list.")
	}
	for _, scope := range strings.Fields(tr.Scope) {
		scope = stripGraphPrefix(scope)
		if strings.EqualFold(scope, ".default") {
			return fail("Microsoft returned .default instead of concrete granted scope names.")
		}
		grant.Scopes = append(grant.Scopes, scope)
	}
	if len(grant.Scopes) == 0 {
		return fail("Microsoft returned no concrete grant scope list; the grant is unknown.")
	}
	grant.Scopes = normalizeScopes(grant.Scopes)
	grant.Status = "known"
	return grant
}

// GrantDifferences reports missing selected workload IDs and extra granted scopes.
// Nil means unknown (including profiles with no recorded workload selection).
func GrantDifferences(workloads []string, grant config.GrantRecord) ([]string, []string) {
	if len(workloads) == 0 || grant.Status != "known" {
		return nil, nil
	}
	missing, extra := []string{}, []string{}
	selected := append([]string{}, BaseDelegatedScopes...)
	for _, id := range workloads {
		w, ok := WorkloadByID(id)
		if !ok {
			missing = append(missing, id)
			continue
		}
		selected = append(selected, w.Scopes...)
		if !ScopesCover(grant.Scopes, w.Scopes) {
			missing = append(missing, id)
		}
	}
	for _, scope := range grant.Scopes {
		if !ScopesCover(selected, []string{scope}) {
			extra = append(extra, scope)
		}
	}
	return missing, extra
}
