package dashboard

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/KilimcininKorOglu/inkwell/internal/crypto"
	"github.com/KilimcininKorOglu/inkwell/internal/models"
	"github.com/KilimcininKorOglu/inkwell/internal/validation"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

var flashMessages = map[string]string{
	"created": "Domain created successfully",
	"updated": "Domain updated successfully",
	"deleted": "Domain deleted successfully",
}

type csrfEntry struct {
	token   string
	created time.Time
}

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	db            *gorm.DB
	tmpl          *template.Template
	encryptionKey string
	csrfTokens    sync.Map
}

// NewRouter creates the Chi router with all dashboard routes.
func NewRouter(db *gorm.DB, templateDir, staticDir, adminUser, adminPassword, encryptionKey string, authDisabled bool) (chi.Router, error) {
	funcMap := template.FuncMap{
		"eq": func(a, b uint) bool {
			return a == b
		},
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseGlob(filepath.Join(templateDir, "**", "*.html"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse partials: %w", err)
	}
	tmpl, err = tmpl.ParseGlob(filepath.Join(templateDir, "*.html"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse layout: %w", err)
	}

	if _, err := tmpl.ParseGlob(filepath.Join(templateDir, "components", "*.html")); err != nil {
		log.Printf("Note: no component templates found: %v", err)
	}

	h := &Handler{db: db, tmpl: tmpl, encryptionKey: encryptionKey}

	r := chi.NewRouter()

	// Static files (no auth required)
	fs := http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir)))
	r.Handle("/static/*", fs)

	// Protected routes
	r.Group(func(r chi.Router) {
		if adminUser != "" && adminPassword != "" {
			r.Use(basicAuth(adminUser, adminPassword))
		}

		// Dashboard routes
		r.Get("/", h.handleIndex)
		r.Get("/dashboard/content", h.handleContent)
		r.Get("/dashboard/detail/{dbid}", h.handleDetail)

		// Domain management routes
		r.Get("/domains", h.handleDomainsList)
		r.Get("/domains/new", h.handleDomainNew)
		r.Post("/domains", h.handleDomainCreate)
		r.Get("/domains/{id}/edit", h.handleDomainEdit)
		r.Post("/domains/{id}", h.handleDomainUpdate)
		r.Post("/domains/{id}/delete", h.handleDomainDelete)
		r.Post("/domains/{id}/toggle", h.handleDomainToggle)
	})

	// Start background CSRF token cleanup (every 5 minutes)
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			h.cleanupExpiredCSRFTokens()
		}
	}()

	return r, nil
}

// --- Dashboard Handlers ---

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	pageData := h.buildPageData(r)
	if err := h.tmpl.ExecuteTemplate(w, "layout.html", pageData); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) handleContent(w http.ResponseWriter, r *http.Request) {
	pageData := h.buildPageData(r)
	if err := h.tmpl.ExecuteTemplate(w, "content", pageData); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) handleDetail(w http.ResponseWriter, r *http.Request) {
	dbidStr := chi.URLParam(r, "dbid")
	dbid, err := strconv.ParseUint(dbidStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid report ID", http.StatusBadRequest)
		return
	}

	detail, err := FetchReportDetail(h.db, uint(dbid))
	if err != nil {
		log.Printf("Error fetching report detail: %v", err)
		http.Error(w, "Report not found", http.StatusNotFound)
		return
	}

	if err := h.tmpl.ExecuteTemplate(w, "detail", detail); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// --- Domain Management Handlers ---

func (h *Handler) handleDomainsList(w http.ResponseWriter, r *http.Request) {
	domains, err := FetchAllDomains(h.db)
	if err != nil {
		log.Printf("Error fetching domains: %v", err)
	}

	data := DomainsPageData{
		Domains:   domains,
		Message:   flashMessages[r.URL.Query().Get("msg")],
		CSRFToken: h.generateCSRFToken(),
	}

	if isHTMX(r) {
		h.render(w, "domainsList", data)
	} else {
		h.render(w, "domainsLayout", data)
	}
}

func (h *Handler) handleDomainNew(w http.ResponseWriter, r *http.Request) {
	data := DomainFormData{
		Domain: DomainFormValues{
			IMAPPort:   993,
			IMAPFolder: "INBOX",
			Enabled:    true,
		},
		IsEdit:    false,
		CSRFToken: h.generateCSRFToken(),
	}

	if isHTMX(r) {
		h.render(w, "domainForm", data)
	} else {
		h.render(w, "domainFormLayout", data)
	}
}

func (h *Handler) handleDomainCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}
	if !h.validateCSRFToken(r) {
		http.Error(w, "Invalid or expired form token", http.StatusForbidden)
		return
	}

	port, portErr := strconv.Atoi(r.FormValue("imap_port"))
	if portErr != nil || port < 1 || port > 65535 {
		h.renderDomainFormError(w, r, "Invalid IMAP port. Must be a number between 1 and 65535.", false)
		return
	}

	// Validate domain name
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		h.renderDomainFormError(w, r, "Domain name is required.", false)
		return
	}
	if len(name) > 253 {
		h.renderDomainFormError(w, r, "Domain name is too long (max 253 characters).", false)
		return
	}

	// Encrypt password
	password := r.FormValue("imap_password")
	if password != "" {
		if h.encryptionKey == "" {
			h.renderDomainFormError(w, r, "Encryption key is not configured. Cannot store IMAP passwords securely.", false)
			return
		}
		encrypted, err := crypto.Encrypt(password, h.encryptionKey)
		if err != nil {
			log.Printf("Error encrypting password: %v", err)
			h.renderDomainFormError(w, r, "Failed to encrypt password", false)
			return
		}
		password = encrypted
	}

	// SSRF protection: validate IMAP server hostname
	imapServer := r.FormValue("imap_server")
	if imapServer != "" && validation.IsPrivateHost(imapServer) {
		h.renderDomainFormError(w, r, "IMAP server hostname cannot resolve to a private or internal address.", false)
		return
	}

	domain := models.Domain{
		Name:              name,
		IMAPServer:        imapServer,
		IMAPPort:          port,
		IMAPUser:          r.FormValue("imap_user"),
		IMAPPassword:      password,
		IMAPFolder:        r.FormValue("imap_folder"),
		IMAPMoveFolder:    r.FormValue("imap_move_folder"),
		IMAPMoveFolderErr: r.FormValue("imap_move_folder_err"),
		Enabled:           r.FormValue("enabled") == "on",
	}

	if err := CreateDomain(h.db, &domain); err != nil {
		log.Printf("Error creating domain: %v", err)
		h.renderDomainFormError(w, r, sanitizeDBError("Failed to create domain.", err), false)
		return
	}

	http.Redirect(w, r, "/domains?msg=created", http.StatusSeeOther)
}

func (h *Handler) handleDomainEdit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	domain, err := FetchDomainByID(h.db, id)
	if err != nil {
		http.Error(w, "Domain not found", http.StatusNotFound)
		return
	}

	data := DomainFormData{
		Domain: DomainFormValues{
			ID:                domain.ID,
			Name:              domain.Name,
			IMAPServer:        domain.IMAPServer,
			IMAPPort:          domain.IMAPPort,
			IMAPUser:          domain.IMAPUser,
			IMAPFolder:        domain.IMAPFolder,
			IMAPMoveFolder:    domain.IMAPMoveFolder,
			IMAPMoveFolderErr: domain.IMAPMoveFolderErr,
			Enabled:           domain.Enabled,
		},
		IsEdit:    true,
		CSRFToken: h.generateCSRFToken(),
	}

	if isHTMX(r) {
		h.render(w, "domainForm", data)
	} else {
		h.render(w, "domainFormLayout", data)
	}
}

func (h *Handler) handleDomainUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}
	if !h.validateCSRFToken(r) {
		http.Error(w, "Invalid or expired form token", http.StatusForbidden)
		return
	}

	existing, err := FetchDomainByID(h.db, id)
	if err != nil {
		http.Error(w, "Domain not found", http.StatusNotFound)
		return
	}

	port, portErr := strconv.Atoi(r.FormValue("imap_port"))
	if portErr != nil || port < 1 || port > 65535 {
		h.renderDomainFormError(w, r, "Invalid IMAP port. Must be a number between 1 and 65535.", true)
		return
	}

	// Keep existing encrypted password if field is blank
	password := existing.IMAPPassword
	if newPass := r.FormValue("imap_password"); newPass != "" {
		if h.encryptionKey == "" {
			h.renderDomainFormError(w, r, "Encryption key is not configured. Cannot store IMAP passwords securely.", true)
			return
		}
		encrypted, err := crypto.Encrypt(newPass, h.encryptionKey)
		if err != nil {
			log.Printf("Error encrypting password: %v", err)
			h.renderDomainFormError(w, r, "Failed to encrypt password", true)
			return
		}
		password = encrypted
	}

	// Validate domain name
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		h.renderDomainFormError(w, r, "Domain name is required.", true)
		return
	}
	if len(name) > 253 {
		h.renderDomainFormError(w, r, "Domain name is too long (max 253 characters).", true)
		return
	}

	// SSRF protection: validate IMAP server hostname
	imapServer := r.FormValue("imap_server")
	if imapServer != "" && validation.IsPrivateHost(imapServer) {
		h.renderDomainFormError(w, r, "IMAP server hostname cannot resolve to a private or internal address.", true)
		return
	}

	existing.Name = name
	existing.IMAPServer = imapServer
	existing.IMAPPort = port
	existing.IMAPUser = r.FormValue("imap_user")
	existing.IMAPPassword = password
	existing.IMAPFolder = r.FormValue("imap_folder")
	existing.IMAPMoveFolder = r.FormValue("imap_move_folder")
	existing.IMAPMoveFolderErr = r.FormValue("imap_move_folder_err")
	existing.Enabled = r.FormValue("enabled") == "on"

	if err := UpdateDomain(h.db, existing); err != nil {
		log.Printf("Error updating domain: %v", err)
		h.renderDomainFormError(w, r, sanitizeDBError("Failed to update domain.", err), true)
		return
	}

	http.Redirect(w, r, "/domains?msg=updated", http.StatusSeeOther)
}

func (h *Handler) handleDomainDelete(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}
	if !h.validateCSRFToken(r) {
		http.Error(w, "Invalid or expired form token", http.StatusForbidden)
		return
	}

	id, err := parseID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := DeleteDomain(h.db, id); err != nil {
		log.Printf("Error deleting domain: %v", err)
		http.Error(w, "Failed to delete domain", http.StatusInternalServerError)
		return
	}

	// Nullify domain_id in reports (orphan gracefully)
	if err := h.db.Model(&models.Report{}).Where("domain_id = ?", id).Update("domain_id", nil).Error; err != nil {
		log.Printf("Error orphaning reports for domain %d: %v", id, err)
	}

	http.Redirect(w, r, "/domains?msg=deleted", http.StatusSeeOther)
}

func (h *Handler) handleDomainToggle(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}
	if !h.validateCSRFToken(r) {
		http.Error(w, "Invalid or expired form token", http.StatusForbidden)
		return
	}

	id, err := parseID(r)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := ToggleDomain(h.db, id); err != nil {
		log.Printf("Error toggling domain: %v", err)
		http.Error(w, "Failed to toggle domain", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/domains", http.StatusSeeOther)
}

// --- Helper Functions ---

func (h *Handler) renderDomainFormError(w http.ResponseWriter, r *http.Request, errMsg string, isEdit bool) {
	port, _ := strconv.Atoi(r.FormValue("imap_port"))
	id, _ := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)

	data := DomainFormData{
		Domain: DomainFormValues{
			ID:                uint(id),
			Name:              r.FormValue("name"),
			IMAPServer:        r.FormValue("imap_server"),
			IMAPPort:          port,
			IMAPUser:          r.FormValue("imap_user"),
			IMAPFolder:        r.FormValue("imap_folder"),
			IMAPMoveFolder:    r.FormValue("imap_move_folder"),
			IMAPMoveFolderErr: r.FormValue("imap_move_folder_err"),
			Enabled:           r.FormValue("enabled") == "on",
		},
		IsEdit:    isEdit,
		Error:     errMsg,
		CSRFToken: h.generateCSRFToken(),
	}

	if isHTMX(r) {
		h.render(w, "domainForm", data)
	} else {
		h.render(w, "domainFormLayout", data)
	}
}

func (h *Handler) buildPageData(r *http.Request) *PageData {
	hasData, err := HasAnyData(h.db)
	if err != nil {
		log.Printf("Error checking data: %v", err)
	}

	opts, err := FetchFilterOptions(h.db)
	if err != nil {
		log.Printf("Error fetching filter options: %v", err)
	}

	searchQuery := r.URL.Query().Get("q")
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")
	domains := r.URL.Query()["domains"]
	orgs := r.URL.Query()["orgs"]
	selectedDomain := r.URL.Query().Get("domain")

	// Selected domain from sidebar overrides multi-select
	if selectedDomain != "" {
		domains = []string{selectedDomain}
	}

	// Default date range: last 7 days
	maxDate := opts.MaxDate
	minDate := opts.MinDate

	maxDateStr := formatDate(maxDate)
	minDateStr := formatDate(minDate)

	if startDateStr == "" {
		defaultStart := maxDate.AddDate(0, 0, -7)
		if defaultStart.Before(minDate) {
			defaultStart = minDate
		}
		startDateStr = formatDate(defaultStart)
	}
	if endDateStr == "" {
		endDateStr = maxDateStr
	}

	// Default: select all domains and orgs
	selectAllDomains := len(domains) == 0
	selectAllOrgs := len(orgs) == 0

	if len(domains) == 0 {
		domains = opts.Domains
	}
	if len(orgs) == 0 {
		orgs = opts.Orgs
	}

	sidebar := SidebarData{
		MinDate:          minDateStr,
		MaxDate:          maxDateStr,
		StartDate:        startDateStr,
		EndDate:          endDateStr,
		AllDomains:       opts.Domains,
		SelectedDomains:  domains,
		SelectAllDomains: selectAllDomains && selectedDomain == "",
		AllOrgs:          opts.Orgs,
		SelectedOrgs:     orgs,
		SelectAllOrgs:    selectAllOrgs,
	}

	pageData := &PageData{
		HasData:        hasData,
		SearchQuery:    searchQuery,
		Sidebar:        sidebar,
		AllDomains:     opts.Domains,
		SelectedDomain: selectedDomain,
	}

	if !hasData || len(domains) == 0 || len(orgs) == 0 {
		return pageData
	}

	var startDate, endDate time.Time
	var parseErr error
	startDate, parseErr = time.ParseInLocation("2006-01-02", startDateStr, time.Local)
	if parseErr != nil {
		startDate = minDate
	}
	endDate, parseErr = time.ParseInLocation("2006-01-02", endDateStr, time.Local)
	if parseErr != nil {
		endDate = maxDate
	}
	if startDate.After(endDate) {
		startDate, endDate = endDate, startDate
	}

	metrics, err := FetchGlobalMetrics(h.db, startDate, endDate, domains, orgs)
	if err != nil {
		log.Printf("Error fetching metrics: %v", err)
	} else {
		pageData.Metrics = metrics
	}

	reports, err := FetchReportsList(h.db, startDate, endDate, domains, orgs, searchQuery)
	if err != nil {
		log.Printf("Error fetching reports: %v", err)
	} else {
		pageData.Reports = reports
	}

	return pageData
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return time.Now().Format("2006-01-02")
	}
	return t.Format("2006-01-02")
}

func parseID(r *http.Request) (uint, error) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}

func sanitizeDBError(generic string, err error) string {
	if strings.Contains(err.Error(), "Duplicate entry") {
		return "A domain with this name already exists."
	}
	return generic + " Please check the input and try again."
}

func (h *Handler) render(w http.ResponseWriter, name string, data interface{}) {
	if err := h.tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) generateCSRFToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		log.Printf("Error generating CSRF token: %v", err)
		return ""
	}
	token := hex.EncodeToString(b)
	h.csrfTokens.Store(token, csrfEntry{token: token, created: time.Now()})
	return token
}

func (h *Handler) validateCSRFToken(r *http.Request) bool {
	token := r.FormValue("_csrf")
	if token == "" {
		return false
	}
	if val, ok := h.csrfTokens.LoadAndDelete(token); ok {
		entry := val.(csrfEntry)
		return time.Since(entry.created) < 1*time.Hour
	}
	return false
}

// cleanupExpiredCSRFTokens removes CSRF tokens older than 1 hour from the store.
func (h *Handler) cleanupExpiredCSRFTokens() {
	now := time.Now()
	h.csrfTokens.Range(func(key, value interface{}) bool {
		entry := value.(csrfEntry)
		if now.Sub(entry.created) > 1*time.Hour {
			h.csrfTokens.Delete(key)
		}
		return true
	})
}

func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// basicAuth returns a middleware that enforces HTTP Basic Authentication.
func basicAuth(expectedUser, expectedPassword string) func(http.Handler) http.Handler {
	expectedUserHash := sha256.Sum256([]byte(expectedUser))
	expectedPassHash := sha256.Sum256([]byte(expectedPassword))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, pass, ok := r.BasicAuth()
			if !ok {
				w.Header().Set("WWW-Authenticate", `Basic realm="Inkwell Dashboard"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			userHash := sha256.Sum256([]byte(user))
			passHash := sha256.Sum256([]byte(pass))

			userMatch := subtle.ConstantTimeCompare(userHash[:], expectedUserHash[:]) == 1
			passMatch := subtle.ConstantTimeCompare(passHash[:], expectedPassHash[:]) == 1

			if !userMatch || !passMatch {
				w.Header().Set("WWW-Authenticate", `Basic realm="Inkwell Dashboard"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
