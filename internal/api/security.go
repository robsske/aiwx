package api

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"github.com/rverhe/aiwx/internal/domain"
)

// requireOrgAccess verifies the caller may act on the given organization.
func (d *Deps) requireOrgAccess(w http.ResponseWriter, r *http.Request, orgID uuid.UUID) bool {
	scope, ok := d.requireScope(w, r)
	if !ok {
		return false
	}
	if orgID != uuid.Nil && !scope.AllowsOrg(orgID) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Organization not found")
		return false
	}
	return true
}

// resolveOrgID picks a target org for creates, enforcing tenant boundaries.
func resolveOrgID(w http.ResponseWriter, scope domain.Scope, requested *uuid.UUID) (uuid.UUID, bool) {
	if requested != nil && *requested != uuid.Nil {
		if !scope.AllowsOrg(*requested) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "Organization not found")
			return uuid.Nil, false
		}
		return *requested, true
	}
	if len(scope.OrgIDs) == 0 {
		if scope.IsPlatformAdmin {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "organization_id is required")
			return uuid.Nil, false
		}
		writeError(w, http.StatusForbidden, "FORBIDDEN", "No organization in scope")
		return uuid.Nil, false
	}
	return scope.OrgIDs[0], true
}

// requireInventoryAccess verifies the caller may act on the inventory's org.
func (d *Deps) requireInventoryAccess(w http.ResponseWriter, r *http.Request, invID uuid.UUID) bool {
	scope, ok := d.requireScope(w, r)
	if !ok {
		return false
	}
	inv, err := d.Repos.Inventories.GetByIDScoped(r.Context(), invID, scope)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Inventory not found")
		return false
	}
	return scope.AllowsOrg(inv.OrganizationID)
}

// requireOrgCredential verifies a credential exists and belongs to orgID.
func (d *Deps) requireOrgCredential(w http.ResponseWriter, r *http.Request, orgID, credID uuid.UUID) bool {
	cred, err := d.Repos.Credentials.GetByID(r.Context(), credID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "scm_credential_id not found")
		return false
	}
	if cred.OrganizationID != orgID {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "scm_credential_id must belong to the project organization")
		return false
	}
	return true
}

// validateSCMURL rejects SSRF / command-injection vectors in git clone URLs.
func validateSCMURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("scm_url is required")
	}
	// Git treats args starting with '-' as options — block option injection.
	if strings.HasPrefix(raw, "-") {
		return fmt.Errorf("scm_url must not start with '-'")
	}
	// Local relative/absolute paths are allowed for dev, but not bare option-like names.
	if strings.Contains(raw, "\x00") {
		return fmt.Errorf("scm_url contains invalid characters")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid scm_url")
	}

	switch strings.ToLower(u.Scheme) {
	case "https", "http", "ssh", "git":
		// ok — network schemes
	case "file", "":
		// file:// or local path: allowed only for explicit file:// or absolute paths (dev).
		// Still reject obviously dangerous forms.
		if strings.HasPrefix(raw, "ext::") {
			return fmt.Errorf("scm_url scheme not allowed")
		}
		if u.Scheme == "file" {
			return nil
		}
		// bare path like /tmp/repo
		if strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "./") || strings.HasPrefix(raw, "../") {
			return nil
		}
		return fmt.Errorf("scm_url scheme must be https, ssh, git, or a local path")
	default:
		// Block git transport helpers such as ext::sh -c ...
		return fmt.Errorf("scm_url scheme %q is not allowed", u.Scheme)
	}

	// Block git remote helpers in the URL itself (ext::, etc.)
	if strings.Contains(raw, "::") {
		return fmt.Errorf("scm_url must not contain transport helpers")
	}

	// For network schemes, reject link-local / metadata endpoints.
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("scm_url host is required")
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("scm_url host is not allowed")
		}
		// Optional: block cloud metadata IP
		if ip.String() == "169.254.169.254" {
			return fmt.Errorf("scm_url host is not allowed")
		}
	}
	return nil
}
